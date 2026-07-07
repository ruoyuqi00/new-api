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

import { useYucoreTranslation } from "../i18n/use-yucore-translation";
import { YucoreBrandMark } from "./yucore-brand-mark";

interface YucorePageHeaderProps {
  eyebrow?: string;
  title: string;
  description?: string;
  actions?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
}

export function YucorePageHeader(props: YucorePageHeaderProps) {
  const { t } = useYucoreTranslation();

  return (
    <header
      className={cn(
        "mx-auto mb-6 grid max-w-6xl gap-6 text-center sm:mb-8",
        props.className,
      )}
    >
      <div className="flex justify-center">
        <YucoreBrandMark />
      </div>
      <div className="mx-auto max-w-3xl">
        {props.eyebrow && (
          <div className="mb-3 inline-flex rounded-full border border-cyan-300/20 bg-cyan-300/10 px-3 py-1 text-xs font-medium text-cyan-100">
            {t(props.eyebrow)}
          </div>
        )}
        <h1 className="text-[clamp(2rem,5.2vw,4.75rem)] leading-[0.98] font-semibold tracking-tight text-white">
          {t(props.title)}
        </h1>
        {props.description && (
          <p className="mx-auto mt-4 max-w-2xl text-sm leading-7 text-white/58 sm:text-base">
            {t(props.description)}
          </p>
        )}
      </div>
      {props.children}
      {props.actions && (
        <div className="flex flex-wrap justify-center gap-3">
          {props.actions}
        </div>
      )}
    </header>
  );
}
