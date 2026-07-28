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
import { Link } from "@tanstack/react-router";
import { ArrowRight, CircuitBoard } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

import { YUCORE_STUDIO_NAME, yucoreStudioModules } from "../data/content";
import { useYucoreTranslation } from "../i18n/use-yucore-translation";

interface YucoreStudioEntryProps {
  className?: string;
  compact?: boolean;
}

const accentClass = {
  cyan: "border-cyan-300/20 bg-cyan-300/10 text-cyan-100",
  violet: "border-violet-300/20 bg-violet-300/10 text-violet-100",
  emerald: "border-emerald-300/20 bg-emerald-300/10 text-emerald-100",
  amber: "border-amber-300/20 bg-amber-300/10 text-amber-100",
};

export function YucoreStudioEntry(props: YucoreStudioEntryProps) {
  const { t } = useYucoreTranslation();

  if (props.compact) {
    return (
      <section
        className={cn(
          "yucore-panel relative overflow-hidden rounded-2xl p-5",
          props.className,
        )}
      >
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_18%_8%,rgba(34,211,238,0.16),transparent_28%),radial-gradient(circle_at_88%_14%,rgba(167,139,250,0.14),transparent_30%)]" />
        <div className="relative flex h-full flex-col gap-5">
          <div>
            <div className="mb-3 inline-flex items-center gap-2 rounded-full border border-cyan-300/20 bg-cyan-300/10 px-3 py-1 text-xs font-medium text-cyan-100">
              <CircuitBoard className="size-3.5" aria-hidden="true" />
              {YUCORE_STUDIO_NAME}
            </div>
            <h2 className="max-w-sm text-2xl leading-tight font-semibold tracking-tight text-white">
              {t("Creative studio, same account core.")}
            </h2>
            <p className="mt-3 max-w-sm text-sm leading-6 text-white/58">
              {t(
                "Bridge image, video, canvas, prompt, and asset workflows without leaving the gateway foundation.",
              )}
            </p>
          </div>

          <div className="grid gap-2 sm:grid-cols-2">
            {yucoreStudioModules.map((module) => {
              const Icon = module.icon;

              return (
                <div
                  key={module.title}
                  className="flex min-w-0 items-center gap-3 rounded-xl border border-white/10 bg-black/25 p-3 backdrop-blur"
                >
                  <span
                    className={cn(
                      "flex size-9 shrink-0 items-center justify-center rounded-xl border",
                      accentClass[module.accent],
                    )}
                  >
                    <Icon className="size-4" aria-hidden="true" />
                  </span>
                  <span className="truncate text-sm font-semibold text-white">
                    {t(module.title)}
                  </span>
                </div>
              );
            })}
          </div>

          <div className="mt-auto flex flex-wrap gap-2">
            <Button
              className="h-9 rounded-xl bg-white text-black hover:bg-cyan-50"
              render={<Link to="/playground/studio" />}
            >
              {t("Open Studio")}
              <ArrowRight data-icon="inline-end" />
            </Button>
            <Button
              variant="outline"
              className="h-9 rounded-xl border-white/15 bg-white/[0.035] text-white hover:bg-white/10"
              render={<Link to="/pricing" />}
            >
              {t("Media models")}
            </Button>
          </div>
        </div>
      </section>
    );
  }

  return (
    <section
      className={cn(
        "yucore-panel relative overflow-hidden rounded-3xl p-5 sm:p-6 lg:p-7",
        props.className,
      )}
    >
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_10%_10%,rgba(34,211,238,0.16),transparent_28%),radial-gradient(circle_at_82%_12%,rgba(250,204,21,0.12),transparent_30%),radial-gradient(circle_at_55%_95%,rgba(244,63,94,0.12),transparent_38%)]" />
      <div className="yucore-studio-grid absolute inset-0 opacity-35" />
      <div className="relative grid gap-7 lg:grid-cols-[0.72fr_1.28fr] lg:items-stretch">
        <div className="flex min-w-0 flex-col justify-between gap-6">
          <div>
            <div className="mb-3 inline-flex items-center gap-2 rounded-full border border-amber-300/20 bg-amber-300/10 px-3 py-1 text-xs font-medium text-amber-100">
              <CircuitBoard className="size-3.5" aria-hidden="true" />
              {YUCORE_STUDIO_NAME}
            </div>
            <h2 className="max-w-xl text-2xl leading-tight font-semibold tracking-tight text-white sm:text-3xl">
              {t("Image, video, and infinite canvas become one studio surface.")}
            </h2>
            <p className="mt-3 max-w-xl text-sm leading-7 text-white/58">
              {t(
                "Bring prompt references, generated media, storyboard notes, render states, and billing visibility into the same YuCore account foundation.",
              )}
            </p>
          </div>
          <div className="grid gap-2">
            {[
              ["Input", "prompt + reference pack"],
              ["Control", "model, size, duration, motion"],
              ["Output", "asset history + quota trace"],
            ].map(([label, value]) => (
              <div
                key={label}
                className="flex items-center justify-between gap-3 rounded-2xl border border-white/10 bg-black/25 px-3 py-2"
              >
                <span className="text-xs text-white/40">{t(label)}</span>
                <span className="truncate text-xs font-semibold text-white/78">
                  {t(value)}
                </span>
              </div>
            ))}
          </div>
          <div className="flex flex-wrap gap-3">
            <Button
              className="h-10 rounded-xl bg-white text-black hover:bg-amber-50"
              render={<Link to="/playground/studio" />}
            >
              {t("Open creative playground")}
              <ArrowRight data-icon="inline-end" />
            </Button>
            <Button
              variant="outline"
              className="h-10 rounded-xl border-white/15 bg-white/[0.035] text-white hover:bg-white/10"
              render={<Link to="/pricing" />}
            >
              {t("View media models")}
            </Button>
          </div>
        </div>

        <div className="yucore-studio-workbench relative min-h-[30rem] overflow-hidden rounded-[1.6rem] border border-white/10 bg-black/35 p-3 backdrop-blur">
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_68%_18%,rgba(250,204,21,0.13),transparent_26%),radial-gradient(circle_at_28%_70%,rgba(34,211,238,0.12),transparent_32%)]" />
          <div className="relative grid h-full gap-3 min-[780px]:grid-cols-[1fr_18rem]">
            <div className="yucore-canvas-preview relative min-h-[26rem] overflow-hidden rounded-[1.25rem] border border-white/10 bg-[#030406]">
              <div className="yucore-canvas-dots absolute inset-0" />
              <div className="yucore-canvas-connection absolute left-[20%] top-[28%] h-px w-[42%]" />
              <div className="yucore-canvas-connection yucore-canvas-connection-b absolute left-[38%] top-[58%] h-px w-[36%]" />
              {yucoreStudioModules.map((module, index) => {
                const Icon = module.icon;
                const nodeClassName = [
                  "left-[9%] top-[14%]",
                  "right-[12%] top-[19%]",
                  "left-[24%] bottom-[16%]",
                  "right-[9%] bottom-[18%]",
                ][index];

                return (
                  <div
                    key={module.title}
                    className={cn(
                      "yucore-canvas-node absolute w-44 rounded-2xl border border-white/10 bg-black/55 p-3 shadow-2xl backdrop-blur",
                      nodeClassName,
                    )}
                    style={{ animationDelay: `${index * 160}ms` }}
                  >
                    <span
                      className={cn(
                        "mb-2 flex size-9 items-center justify-center rounded-xl border",
                        accentClass[module.accent],
                      )}
                    >
                      <Icon className="size-4" aria-hidden="true" />
                    </span>
                    <h3 className="truncate text-sm font-semibold text-white">
                      {t(module.title)}
                    </h3>
                    <p className="mt-1 line-clamp-2 text-xs leading-5 text-white/48">
                      {t(module.description)}
                    </p>
                  </div>
                );
              })}
              <div className="yucore-canvas-compose absolute left-1/2 top-1/2 flex size-28 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border border-amber-200/20 bg-amber-200/10 text-center text-xs font-semibold text-amber-50">
                {t("compose")}
              </div>
            </div>

            <aside className="grid gap-3">
              {[
                ["Image render", "Queued", "cyan"],
                ["Video motion", "Calibrating", "violet"],
                ["Canvas save", "Synced", "emerald"],
              ].map(([label, value, tone]) => (
                <div
                  key={label}
                  className="rounded-2xl border border-white/10 bg-white/[0.035] p-3"
                >
                  <div className="mb-3 flex items-center justify-between gap-3">
                    <span className="text-xs font-semibold text-white">
                      {t(label)}
                    </span>
                    <span
                      className={cn(
                        "rounded-full border px-2 py-1 text-[10px]",
                        tone === "cyan" &&
                          "border-cyan-300/20 bg-cyan-300/10 text-cyan-100",
                        tone === "violet" &&
                          "border-violet-300/20 bg-violet-300/10 text-violet-100",
                        tone === "emerald" &&
                          "border-emerald-300/20 bg-emerald-300/10 text-emerald-100",
                      )}
                    >
                      {t(value)}
                    </span>
                  </div>
                  <div className="yucore-render-progress h-1.5 overflow-hidden rounded-full bg-white/10">
                    <span className="block h-full rounded-full bg-gradient-to-r from-cyan-200 via-amber-200 to-rose-300" />
                  </div>
                </div>
              ))}
              <div className="rounded-2xl border border-white/10 bg-black/35 p-3">
                <div className="mb-2 text-xs font-semibold text-white">
                  {t("Prompt memory")}
                </div>
                <p className="text-xs leading-6 text-white/52">
                  {t(
                    "Store reusable prompts, image references, storyboard beats, and generated assets for the next run.",
                  )}
                </p>
              </div>
            </aside>
          </div>
        </div>
      </div>
    </section>
  );
}
