/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { createHash } from "node:crypto";
import { mkdir, mkdtemp, readFile, rm, stat, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, test } from "bun:test";

import {
  VIDEO_SAMPLES,
  importVideoSamples,
  rollbackVideoSamples,
} from "./import-yucore-video-samples.mjs";

const EXPECTED_VIDEO_SAMPLES = [
  ["happyhouse-1.0.mp4", "happyhouse-1.0"],
  ["happyhouse-1.1.mp4", "happyhouse-1.1"],
  ["minimax-h3-2k.mp4", "minimax-h3-2k"],
  ["omni-fast-no-water.mp4", "omni-fast-no-water"],
  ["omni-fast.mp4", "omni-fast"],
  ["omni-v2v-no-water.mp4", "omni-v2v-no-water"],
  ["omni-v2v.mp4", "omni-v2v"],
  ["sd7-seedance-2.0-1080p.mp4", "sd7-seedance-2.0-1080p"],
  ["sd7-seedance-2.0-720p.mp4", "sd7-seedance-2.0-720p"],
  ["seedance-2.0.mp4", "seedance-2.0"],
];

async function createSourceDirectory(root) {
  const sourceDir = path.join(root, "source");
  await mkdir(sourceDir, { recursive: true });
  for (const [index, [fileName]] of EXPECTED_VIDEO_SAMPLES.entries()) {
    await writeFile(path.join(sourceDir, fileName), `video-${index}\n`);
  }
  return sourceDir;
}

function successResponse(taskID, form) {
  const file = form.get("file");
  return new Response(
    JSON.stringify({
      success: true,
      data: {
        created: true,
        task_id: taskID,
        asset_url: `/api/yucore/media/tasks/${taskID}/assets/0`,
        sha256: form.get("sha256"),
        size: file.size,
      },
    }),
    { status: 200, headers: { "Content-Type": "application/json" } },
  );
}

describe("YuCore production video sample importer", () => {
  test("uses the exact approved ten-file manifest", () => {
    expect(VIDEO_SAMPLES).toEqual(EXPECTED_VIDEO_SAMPLES);
  });

  test("rejects insecure remote URLs and result files inside the worktree", async () => {
    await expect(
      importVideoSamples({
        baseUrl: "http://example.com",
        sourceDir: tmpdir(),
        resultFile: path.join(tmpdir(), "unused-video-sample-result.json"),
        authHeader: "Authorization: Bearer secret",
      }),
    ).rejects.toThrow("base URL must use HTTPS unless it is loopback");

    const resultFile = path.join(
      path.dirname(fileURLToPath(import.meta.url)),
      ".forbidden-video-sample-result.json",
    );
    await expect(
      importVideoSamples({
        baseUrl: "http://127.0.0.1:3000",
        sourceDir: tmpdir(),
        resultFile,
        authHeader: "Authorization: Bearer secret",
      }),
    ).rejects.toThrow("result file must be outside the Git worktree");
  });

  test("imports one file at a time in order and writes a redacted 0600 result", async () => {
    const root = await mkdtemp(path.join(tmpdir(), "yucore-sample-import-"));
    try {
      const sourceDir = await createSourceDirectory(root);
      const resultFile = path.join(root, "result.json");
      const authHeader = "Authorization: Bearer never-log-this-value";
      const logs = [];
      const calls = [];
      let active = 0;
      let maxActive = 0;
      const fetchImpl = async (url, options) => {
        active += 1;
        maxActive = Math.max(maxActive, active);
        const form = options.body;
        const file = form.get("file");
        const bytes = Buffer.from(await file.arrayBuffer());
        calls.push({
          url,
          modelID: form.get("model_id"),
          fileName: file.name,
          checksum: form.get("sha256"),
          uploadedChecksum: createHash("sha256").update(bytes).digest("hex"),
          size: file.size,
          authorization: options.headers.get("Authorization"),
        });
        await Promise.resolve();
        active -= 1;
        return successResponse(`yu_sample_task_${calls.length}`, form);
      };

      const result = await importVideoSamples({
        baseUrl: "http://127.0.0.1:3000",
        sourceDir,
        resultFile,
        authHeader,
        fetchImpl,
        log: (message) => logs.push(message),
      });

      expect(maxActive).toBe(1);
      expect(calls.map((call) => [call.fileName, call.modelID])).toEqual(EXPECTED_VIDEO_SAMPLES);
      for (const [index, call] of calls.entries()) {
        const expectedBytes = Buffer.from(`video-${index}\n`);
        expect(call.checksum).toBe(createHash("sha256").update(expectedBytes).digest("hex"));
        expect(call.uploadedChecksum).toBe(call.checksum);
        expect(call.size).toBe(expectedBytes.length);
        expect(call.authorization).toBe("Bearer never-log-this-value");
      }
      expect(result.results).toHaveLength(10);
      for (const [index, row] of result.results.entries()) {
        expect(row).toEqual({
          file_name: EXPECTED_VIDEO_SAMPLES[index][0],
          managed_file_name: `sample_${calls[index].checksum}.mp4`,
          model_id: EXPECTED_VIDEO_SAMPLES[index][1],
          sha256: calls[index].checksum,
          size: calls[index].size,
          task_id: `yu_sample_task_${index + 1}`,
          created: true,
        });
      }
      const resultText = await readFile(resultFile, "utf8");
      expect(resultText).not.toContain(authHeader);
      expect(resultText).not.toContain("never-log-this-value");
      expect(logs.join("\n")).not.toContain(authHeader);
      expect(logs.join("\n")).not.toContain("never-log-this-value");
      if (process.platform !== "win32") {
        const resultStat = await stat(resultFile);
        expect(resultStat.mode & 0o777).toBe(0o600);
      }
    } finally {
      await rm(root, { recursive: true, force: true });
    }
  });

  test("stops after the first server failure and preserves prior results", async () => {
    const root = await mkdtemp(path.join(tmpdir(), "yucore-sample-stop-"));
    try {
      const sourceDir = await createSourceDirectory(root);
      const resultFile = path.join(root, "result.json");
      const calls = [];
      const logs = [];
      await expect(
        importVideoSamples({
          baseUrl: "http://localhost:3000",
          sourceDir,
          resultFile,
          authHeader: "Cookie: session=secret-session",
          fetchImpl: async (_url, options) => {
            calls.push(options.body.get("model_id"));
            if (calls.length === 2) {
              return new Response(JSON.stringify({ success: false, message: "rejected" }), {
                status: 200,
                headers: { "Content-Type": "application/json" },
              });
            }
            return successResponse("first-task", options.body);
          },
          log: (message) => logs.push(message),
        }),
      ).rejects.toThrow("happyhouse-1.1.mp4");

      expect(calls).toEqual(["happyhouse-1.0", "happyhouse-1.1"]);
      const partial = JSON.parse(await readFile(resultFile, "utf8"));
      expect(partial.results.map((row) => row.task_id)).toEqual(["first-task"]);
      expect(logs.join("\n")).not.toContain("secret-session");
    } finally {
      await rm(root, { recursive: true, force: true });
    }
  });

  test("rolls back only newly created manifest tasks in reverse order", async () => {
    const root = await mkdtemp(path.join(tmpdir(), "yucore-sample-rollback-"));
    try {
      const manifestPath = path.join(root, "result.json");
      await writeFile(
        manifestPath,
        JSON.stringify({
          results: [
            { task_id: "task-one", created: true },
            { task_id: "preexisting-task", created: false },
            { task_id: "task-three", created: true },
          ],
        }),
      );
      const deleted = [];
      const logs = [];
      await rollbackVideoSamples({
        baseUrl: "http://[::1]:3000",
        manifestPath,
        authHeader: "Authorization: Bearer rollback-secret",
        fetchImpl: async (url, options) => {
          deleted.push({ url, method: options.method });
          return new Response(JSON.stringify({ success: true, data: null }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          });
        },
        log: (message) => logs.push(message),
      });

      expect(deleted).toEqual([
        {
          url: "http://[::1]:3000/api/yucore/media/admin/sample-assets/task-three",
          method: "DELETE",
        },
        {
          url: "http://[::1]:3000/api/yucore/media/admin/sample-assets/task-one",
          method: "DELETE",
        },
      ]);
      expect(logs.join("\n")).not.toContain("rollback-secret");
    } finally {
      await rm(root, { recursive: true, force: true });
    }
  });
});
