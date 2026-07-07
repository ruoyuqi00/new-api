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
import { cn } from "@/lib/utils";

import { yucoreMetrics } from "../data/content";
import { useYucoreTranslation } from "../i18n/use-yucore-translation";

interface YucoreMetricStripProps {
  className?: string;
}

export function YucoreMetricStrip(props: YucoreMetricStripProps) {
  const { t } = useYucoreTranslation();

  return (
    <div
      className={cn(
        "grid grid-cols-2 overflow-hidden rounded-2xl border border-white/10 bg-white/[0.035] backdrop-blur md:grid-cols-4",
        props.className,
      )}
    >
      {yucoreMetrics.map((metric, index) => (
        <div
          key={metric.label}
          className={cn(
            "border-white/10 px-4 py-5 text-center md:px-6",
            index % 2 === 0 && "max-md:border-r",
            index < 2 && "max-md:border-b",
            index < yucoreMetrics.length - 1 && "md:border-r",
          )}
        >
          <div className="text-2xl font-semibold tracking-tight text-white">
            {metric.value}
          </div>
          <div className="mt-1 text-xs text-white/45">{t(metric.label)}</div>
        </div>
      ))}
    </div>
  );
}
