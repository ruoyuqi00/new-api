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
import { CreditCard, KeyRound, ShieldCheck, WandSparkles } from "lucide-react";

import { cn } from "@/lib/utils";

import { useYucoreTranslation } from "../i18n/use-yucore-translation";

const accessSteps = [
  {
    title: "Identity",
    detail: "session protected",
    icon: ShieldCheck,
    accent: "cyan",
  },
  {
    title: "Wallet",
    detail: "quota linked",
    icon: CreditCard,
    accent: "amber",
  },
  {
    title: "API Keys",
    detail: "scopes ready",
    icon: KeyRound,
    accent: "emerald",
  },
  {
    title: "Studio",
    detail: "media unlocked",
    icon: WandSparkles,
    accent: "rose",
  },
] as const;

const accentClassName = {
  cyan: "border-cyan-300/25 bg-cyan-300/10 text-cyan-100",
  amber: "border-amber-300/25 bg-amber-300/10 text-amber-100",
  emerald: "border-emerald-300/25 bg-emerald-300/10 text-emerald-100",
  rose: "border-rose-300/25 bg-rose-300/10 text-rose-100",
} as const;

interface YucoreAccessCoreProps {
  className?: string;
}

export function YucoreAccessCore(props: YucoreAccessCoreProps) {
  const { t } = useYucoreTranslation();

  return (
    <div className={cn("yucore-access-core relative", props.className)}>
      <div className="yucore-access-field relative overflow-hidden rounded-[1.5rem] border border-white/10 bg-black/35 p-3 backdrop-blur-2xl">
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_52%_34%,rgba(34,211,238,0.16),transparent_24%),radial-gradient(circle_at_74%_72%,rgba(250,204,21,0.12),transparent_30%)]" />
        <div className="relative flex min-h-[17.5rem] items-center justify-center rounded-[1.1rem] border border-white/10 bg-[#030406]">
          <div className="yucore-access-grid absolute inset-0" />
          <div className="yucore-access-ring absolute size-52 rounded-full" />
          <div className="yucore-access-ring yucore-access-ring-secondary absolute size-64 rounded-full" />
          <div className="relative flex size-30 items-center justify-center rounded-full border border-white/15 bg-white/[0.035]">
            <div className="yucore-access-core-dot flex size-20 flex-col items-center justify-center rounded-full text-center">
              <ShieldCheck
                className="mb-1.5 size-6 text-white"
                aria-hidden="true"
              />
              <span className="text-xs font-semibold text-white">
                {t("Access Core")}
              </span>
              <span className="mt-1 text-[10px] text-white/45">
                {t("online")}
              </span>
            </div>
          </div>

          {accessSteps.map((step, index) => {
            const Icon = step.icon;
            const position = [
              "top-3 left-3",
              "top-7 right-3",
              "bottom-4 left-4",
              "right-4 bottom-4",
            ][index];

            return (
              <div
                key={step.title}
                className={cn(
                  "yucore-access-node absolute w-36 rounded-2xl border bg-black/50 p-2.5 backdrop-blur",
                  position,
                  accentClassName[step.accent],
                )}
                style={{ animationDelay: `${index * 160}ms` }}
              >
                <div className="mb-2 flex items-center gap-2">
                  <Icon className="size-4" aria-hidden="true" />
                  <span className="text-xs font-semibold">{t(step.title)}</span>
                </div>
                <div className="text-[11px] text-white/52">
                  {t(step.detail)}
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
