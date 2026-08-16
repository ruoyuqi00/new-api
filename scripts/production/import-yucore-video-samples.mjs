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
import { createHash, randomUUID } from "node:crypto";
import { chmod, mkdir, open, readFile, rename, stat, unlink } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

export const VIDEO_SAMPLES = [
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

const COLLECTION_ID = "video-model-examples";
const WORKTREE_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");

function normalizeBaseUrl(value) {
  let url;
  try {
    url = new URL(value);
  } catch {
    throw new Error("base URL is invalid");
  }
  const hostname = url.hostname.toLowerCase();
  const loopback =
    hostname === "localhost" ||
    hostname === "::1" ||
    hostname === "[::1]" ||
    hostname.startsWith("127.");
  if (
    url.username ||
    url.password ||
    url.search ||
    url.hash ||
    (url.protocol !== "https:" && !(url.protocol === "http:" && loopback))
  ) {
    throw new Error("base URL must use HTTPS unless it is loopback");
  }
  return url.toString().replace(/\/$/, "");
}

function parseAuthHeader(value) {
  if (typeof value !== "string" || value.includes("\r") || value.includes("\n")) {
    throw new Error("YUAPI_ADMIN_AUTH_HEADER is invalid");
  }
  const separator = value.indexOf(":");
  const name = value.slice(0, separator).trim();
  const headerValue = value.slice(separator + 1).trim();
  if (separator <= 0 || !/^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/.test(name) || headerValue === "") {
    throw new Error("YUAPI_ADMIN_AUTH_HEADER is invalid");
  }
  const headers = new Headers();
  try {
    headers.set(name, headerValue);
  } catch {
    throw new Error("YUAPI_ADMIN_AUTH_HEADER is invalid");
  }
  return headers;
}

function isPathInside(parent, candidate) {
  let parentPath = path.resolve(parent);
  let candidatePath = path.resolve(candidate);
  if (process.platform === "win32") {
    parentPath = parentPath.toLowerCase();
    candidatePath = candidatePath.toLowerCase();
  }
  const relative = path.relative(parentPath, candidatePath);
  return (
    relative === "" ||
    (!relative.startsWith(`..${path.sep}`) && relative !== ".." && !path.isAbsolute(relative))
  );
}

async function writeResultManifest(resultFile, manifest) {
  const finalPath = path.resolve(resultFile);
  if (isPathInside(WORKTREE_ROOT, finalPath)) {
    throw new Error("result file must be outside the Git worktree");
  }
  const directory = path.dirname(finalPath);
  const tempPath = path.join(
    directory,
    `.${path.basename(finalPath)}.${process.pid}.${randomUUID()}.tmp`,
  );
  let handle;
  try {
    await mkdir(directory, { recursive: true, mode: 0o700 });
    handle = await open(tempPath, "wx", 0o600);
    await handle.writeFile(`${JSON.stringify(manifest, null, 2)}\n`, "utf8");
    await handle.sync();
    await handle.close();
    handle = undefined;
    await chmod(tempPath, 0o600);
    await rename(tempPath, finalPath);
    await chmod(finalPath, 0o600);
  } catch {
    if (handle) await handle.close().catch(() => {});
    await unlink(tempPath).catch(() => {});
    throw new Error("result manifest could not be written");
  }
}

async function parseSuccessResponse(response, fileName) {
  if (!response.ok) {
    throw new Error(`${fileName}: server returned HTTP ${response.status}`);
  }
  let payload;
  try {
    payload = await response.json();
  } catch {
    throw new Error(`${fileName}: server returned invalid JSON`);
  }
  if (!payload || payload.success !== true) {
    throw new Error(`${fileName}: server rejected the sample import`);
  }
  return payload.data;
}

export async function importVideoSamples({
  baseUrl,
  sourceDir,
  resultFile,
  authHeader,
  fetchImpl = fetch,
  log = console.log,
}) {
  const normalizedBaseUrl = normalizeBaseUrl(baseUrl);
  const headers = parseAuthHeader(authHeader);
  if (!sourceDir || !resultFile) {
    throw new Error("source directory and result file are required");
  }
  if (isPathInside(WORKTREE_ROOT, resultFile)) {
    throw new Error("result file must be outside the Git worktree");
  }
  const manifest = {
    version: 1,
    collection_id: COLLECTION_ID,
    created_at: new Date().toISOString(),
    results: [],
  };
  await writeResultManifest(resultFile, manifest);

  for (const [index, [fileName, modelID]] of VIDEO_SAMPLES.entries()) {
    const sourcePath = path.join(path.resolve(sourceDir), fileName);
    let bytes;
    let fileStat;
    try {
      const source = await Promise.all([readFile(sourcePath), stat(sourcePath)]);
      bytes = source[0];
      fileStat = source[1];
    } catch {
      throw new Error(`${fileName}: source video is unavailable`);
    }
    if (!fileStat.isFile() || fileStat.size <= 0 || fileStat.size !== bytes.length) {
      throw new Error(`${fileName}: source video is invalid`);
    }
    const checksum = createHash("sha256").update(bytes).digest("hex");
    const form = new FormData();
    form.append("collection_id", COLLECTION_ID);
    form.append("model_id", modelID);
    form.append("sha256", checksum);
    form.append("file", new Blob([bytes], { type: "video/mp4" }), fileName);
    log(`Importing ${index + 1}/${VIDEO_SAMPLES.length}: ${fileName}`);

    let response;
    try {
      response = await fetchImpl(`${normalizedBaseUrl}/api/yucore/media/admin/sample-assets`, {
        method: "POST",
        headers,
        body: form,
        redirect: "error",
      });
    } catch {
      throw new Error(`${fileName}: sample import request failed`);
    }
    const data = await parseSuccessResponse(response, fileName);
    if (
      !data ||
      typeof data.task_id !== "string" ||
      data.task_id === "" ||
      typeof data.created !== "boolean" ||
      data.sha256 !== checksum ||
      data.size !== fileStat.size
    ) {
      throw new Error(`${fileName}: server returned inconsistent import data`);
    }
    manifest.results.push({
      file_name: fileName,
      managed_file_name: `sample_${checksum}.mp4`,
      model_id: modelID,
      sha256: checksum,
      size: fileStat.size,
      task_id: data.task_id,
      created: data.created,
    });
    await writeResultManifest(resultFile, manifest);
    log(`Imported ${fileName}: ${data.task_id}`);
  }

  return manifest;
}

export async function rollbackVideoSamples({
  baseUrl,
  manifestPath,
  authHeader,
  fetchImpl = fetch,
  log = console.log,
}) {
  const normalizedBaseUrl = normalizeBaseUrl(baseUrl);
  const headers = parseAuthHeader(authHeader);
  let manifest;
  try {
    manifest = JSON.parse(await readFile(path.resolve(manifestPath), "utf8"));
  } catch {
    throw new Error("rollback manifest could not be read");
  }
  if (!manifest || !Array.isArray(manifest.results)) {
    throw new Error("rollback manifest is invalid");
  }
  const taskIDs = manifest.results
    .filter((row) => row && row.created === true)
    .map((row) => row.task_id);
  if (
    taskIDs.some((taskID) => typeof taskID !== "string" || taskID.trim() === "") ||
    new Set(taskIDs).size !== taskIDs.length
  ) {
    throw new Error("rollback manifest is invalid");
  }

  for (const taskID of taskIDs.reverse()) {
    let response;
    try {
      response = await fetchImpl(
        `${normalizedBaseUrl}/api/yucore/media/admin/sample-assets/${encodeURIComponent(taskID)}`,
        { method: "DELETE", headers, redirect: "error" },
      );
    } catch {
      throw new Error(`${taskID}: sample rollback request failed`);
    }
    await parseSuccessResponse(response, taskID);
    log(`Rolled back ${taskID}`);
  }

  return { rolled_back: taskIDs };
}

function parseArguments(argv) {
  const parsed = {};
  for (let index = 0; index < argv.length; index += 2) {
    const key = argv[index];
    const value = argv[index + 1];
    if (!key?.startsWith("--") || !value || value.startsWith("--")) {
      throw new Error("invalid command arguments");
    }
    const name = key.slice(2);
    if (parsed[name] !== undefined) {
      throw new Error("duplicate command argument");
    }
    parsed[name] = value;
  }
  return parsed;
}

async function main() {
  const args = parseArguments(process.argv.slice(2));
  const allowed = new Set(["base-url", "source-dir", "result-file", "rollback-manifest"]);
  if (Object.keys(args).some((key) => !allowed.has(key))) {
    throw new Error("unsupported command argument");
  }
  if (!args["base-url"]) {
    throw new Error("--base-url is required");
  }
  const authHeader = process.env.YUAPI_ADMIN_AUTH_HEADER;
  if (args["rollback-manifest"]) {
    if (args["source-dir"] || args["result-file"]) {
      throw new Error("rollback mode does not accept import arguments");
    }
    await rollbackVideoSamples({
      baseUrl: args["base-url"],
      manifestPath: args["rollback-manifest"],
      authHeader,
    });
    return;
  }
  if (!args["source-dir"] || !args["result-file"]) {
    throw new Error("--source-dir and --result-file are required");
  }
  await importVideoSamples({
    baseUrl: args["base-url"],
    sourceDir: args["source-dir"],
    resultFile: args["result-file"],
    authHeader,
  });
}

const isMain =
  process.argv[1] && pathToFileURL(path.resolve(process.argv[1])).href === import.meta.url;
if (isMain) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.message : "video sample operation failed");
    process.exitCode = 1;
  });
}
