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
import { Boxes, Gauge, RadioTower, Route } from "lucide-react";

import { cn } from "@/lib/utils";

import { useYucoreTranslation } from "../i18n/use-yucore-translation";

interface YucoreModelHubRadarProps {
  className?: string;
  groupCount: number;
  modelCount: number;
  vendorCount: number;
}

export function YucoreModelHubRadar(props: YucoreModelHubRadarProps) {
  const { t } = useYucoreTranslation();
  const stats = [
    {
      label: "Models",
      value: String(props.modelCount),
      icon: Boxes,
      className: "left-4 top-16",
    },
    {
      label: "Vendors",
      value: String(props.vendorCount),
      icon: RadioTower,
      className: "right-4 top-16",
    },
    {
      label: "Groups",
      value: String(props.groupCount),
      icon: Route,
      className: "bottom-20 left-7",
    },
    {
      label: "Pricing",
      value: "live",
      icon: Gauge,
      className: "bottom-20 right-6",
    },
  ];
  const lanes = [
    ["routing", "ready"],
    ["rates", "indexed"],
    ["filters", "armed"],
  ];

  return (
    <aside
      className={cn(
        "yucore-model-radar relative min-h-[24rem] overflow-hidden rounded-[1.6rem] border border-white/10 bg-black/35 p-4 text-left backdrop-blur-2xl",
        props.className,
      )}
    >
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_50%_38%,rgba(34,211,238,0.16),transparent_28%),radial-gradient(circle_at_70%_76%,rgba(250,204,21,0.13),transparent_34%)]" />
      <div className="yucore-model-radar-grid absolute inset-0" />
      <div className="yucore-ops-scan absolute inset-3 rounded-[1.25rem]" />
      <div className="absolute top-4 left-4 right-4 flex items-center justify-between gap-3">
        <div>
          <div className="text-[10px] font-medium tracking-[0.24em] text-cyan-100/48 uppercase">
            {t("Model intelligence")}
          </div>
          <div className="mt-1 text-sm font-semibold text-white">
            {t("live index")}
          </div>
        </div>
        <span className="rounded-full border border-cyan-200/20 bg-cyan-300/10 px-2.5 py-1 text-[11px] font-medium text-cyan-100">
          {t("scanning")}
        </span>
      </div>
      <div className="absolute left-1/2 top-1/2 flex size-40 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border border-white/10 bg-white/[0.035]">
        <div className="yucore-model-radar-core flex size-28 flex-col items-center justify-center rounded-full text-center">
          <Boxes className="mb-2 size-7 text-white" aria-hidden="true" />
          <span className="text-xs font-semibold text-white">
            {t("Model Hub")}
          </span>
          <span className="mt-1 text-[10px] text-white/45">
            {t("routing ready")}
          </span>
        </div>
      </div>
      <div className="yucore-model-radar-ring absolute left-1/2 top-1/2 size-72 -translate-x-1/2 -translate-y-1/2 rounded-full" />
      <div className="yucore-model-radar-ring yucore-model-radar-ring-wide absolute left-1/2 top-1/2 size-96 -translate-x-1/2 -translate-y-1/2 rounded-full" />
      <div className="yucore-model-radar-sweep absolute left-1/2 top-1/2 h-px w-[72%] -translate-x-1/2 -translate-y-1/2" />
      <div className="yucore-canvas-flow absolute left-[32%] top-[36%] size-2 rounded-full bg-cyan-100" />
      <div className="yucore-canvas-flow absolute right-[30%] top-[62%] size-2 rounded-full bg-amber-100 [animation-delay:420ms]" />

      {stats.map((stat, index) => {
        const Icon = stat.icon;

        return (
          <div
            key={stat.label}
            className={cn(
              "yucore-model-radar-card absolute w-32 rounded-2xl border border-white/10 bg-black/50 p-3 backdrop-blur",
              stat.className,
            )}
            style={{ animationDelay: `${index * 140}ms` }}
          >
            <div className="mb-2 flex items-center gap-2 text-xs text-white/52">
              <Icon className="size-3.5 text-cyan-100" aria-hidden="true" />
              {t(stat.label)}
            </div>
            <div className="text-xl font-semibold text-white">
              {t(stat.value)}
            </div>
          </div>
        );
      })}
      <div className="absolute inset-x-4 bottom-4 grid gap-2 sm:grid-cols-3">
        {lanes.map(([label, value], index) => (
          <div
            key={label}
            className="rounded-xl border border-white/10 bg-black/45 px-3 py-2"
          >
            <div className="mb-1 flex items-center justify-between gap-2 text-[10px] text-white/42">
              <span>{t(label)}</span>
              <span>{t(value)}</span>
            </div>
            <div className="h-1 overflow-hidden rounded-full bg-white/10">
              <span
                className="yucore-ops-lane block h-full rounded-full bg-linear-to-r from-cyan-200 via-amber-100 to-emerald-200"
                style={{ animationDelay: `${index * 170}ms` }}
              />
            </div>
          </div>
        ))}
      </div>
    </aside>
  );
}
