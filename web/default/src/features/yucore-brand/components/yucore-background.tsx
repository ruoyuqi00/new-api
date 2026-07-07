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

import { YucoreMotionCanvas } from "./yucore-motion-canvas";
import { YucoreWebglEarth } from "./yucore-webgl-earth";

interface YucoreBackgroundProps {
  active?: boolean;
  className?: string;
  coreMode?: "full" | "ambient";
  intensity?: "calm" | "hero" | "workbench";
  showEarthCore?: boolean;
}

export function YucoreBackground(props: YucoreBackgroundProps) {
  const intensity = props.intensity ?? "calm";
  const showMotionCanvas = props.active !== false;

  return (
    <div
      aria-hidden="true"
      className={cn(
        "pointer-events-none absolute inset-0 z-0 overflow-hidden",
        props.className,
      )}
    >
      <div
        className={cn(
          "absolute inset-0 z-0 bg-[radial-gradient(circle_at_72%_32%,rgba(244,63,94,0.16),transparent_23%),linear-gradient(118deg,rgba(34,211,238,0.18)_0%,transparent_28%),linear-gradient(242deg,rgba(250,204,21,0.12)_0%,transparent_31%),linear-gradient(135deg,#020304_0%,#06070b_45%,#10120d_100%)]",
          intensity === "calm" && "opacity-85",
          intensity === "hero" && "opacity-100",
          intensity === "workbench" && "opacity-95",
        )}
      />
      {showMotionCanvas && (
        <YucoreMotionCanvas
          active={props.active}
          coreMode={props.coreMode}
          intensity={intensity}
          className={cn(
            "z-[2] mix-blend-screen",
            intensity === "calm" && "opacity-[0.7]",
            intensity === "hero" && "opacity-100",
            intensity === "workbench" && "opacity-[0.9]",
          )}
        />
      )}
      <div
        className={cn(
          "yucore-background-particle-mesh absolute inset-0 z-[2]",
          intensity === "hero" && "yucore-background-particle-mesh-hero",
          intensity === "workbench" && "yucore-background-particle-mesh-workbench",
          intensity === "calm" && "yucore-background-particle-mesh-calm",
        )}
      />
      {showMotionCanvas && props.showEarthCore !== false && (
        <div
          className={cn(
            "yucore-background-earth-core absolute z-[3]",
            intensity === "calm" && "yucore-background-earth-core-calm",
            intensity === "hero" && "yucore-background-earth-core-hero",
            intensity === "workbench" && "yucore-background-earth-core-workbench",
          )}
        >
          <YucoreWebglEarth
            active={props.active}
            density={intensity === "hero" ? "loader" : "persistent"}
          />
        </div>
      )}
      <div
        className={cn(
          "yucore-energy-vortex absolute top-[18%] left-1/2 z-[1] h-[38rem] w-[38rem] -translate-x-1/2 rounded-full",
          intensity === "hero" ? "opacity-70" : "opacity-40",
        )}
      />
      <div
        className={cn(
          "yucore-energy-thread yucore-energy-thread-a absolute top-[28%] left-[-12%] z-[4] h-px w-[124vw]",
          intensity === "calm" && "opacity-45",
        )}
      />
      <div
        className={cn(
          "yucore-energy-thread yucore-energy-thread-b absolute top-[58%] left-[-18%] z-[4] h-px w-[132vw]",
          intensity === "workbench" ? "opacity-70" : "opacity-52",
        )}
      />
      <div
        className={cn(
          "yucore-background-power-field absolute inset-0 z-[3]",
          intensity === "hero" && "yucore-background-power-field-hero",
          intensity === "workbench" && "yucore-background-power-field-workbench",
          intensity === "calm" && "yucore-background-power-field-calm",
        )}
      />
      <div
        className={cn(
          "yucore-grid absolute inset-0 z-[1]",
          intensity === "hero" ? "opacity-[0.36]" : "opacity-[0.28]",
        )}
      />
      <div className="yucore-scanlines absolute inset-0 z-[4] opacity-[0.18]" />
      <div className="absolute inset-0 z-[4] bg-[radial-gradient(circle_at_50%_45%,transparent_0%,transparent_43%,rgba(0,0,0,0.46)_100%)]" />
      <div className="absolute inset-x-[-12%] bottom-[-18%] z-[4] h-[48%] rotate-[-2deg] bg-[linear-gradient(90deg,transparent,rgba(34,211,238,0.13),rgba(250,204,21,0.13),transparent)] opacity-70 blur-xl" />
      <div className="absolute top-[8%] left-1/2 z-[4] h-px w-[68rem] max-w-[95vw] -translate-x-1/2 bg-[linear-gradient(90deg,transparent,rgba(103,232,249,0.48),rgba(250,204,21,0.32),transparent)]" />
    </div>
  );
}
