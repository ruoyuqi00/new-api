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
import { useEffect, useRef } from "react";

import { cn } from "@/lib/utils";

interface YucoreMotionCanvasProps {
  active?: boolean;
  className?: string;
  coreMode?: "full" | "ambient";
  intensity?: "calm" | "hero" | "workbench";
}

type Particle = {
  cluster: number;
  energy: number;
  homeX: number;
  homeY: number;
  lane: number;
  noise: number;
  orbit: boolean;
  x: number;
  y: number;
  vx: number;
  vy: number;
  r: number;
  phase: number;
  spin: number;
  tone: "cyan" | "amber" | "violet";
};

type ShardParticle = {
  angle: number;
  depth: number;
  energy: number;
  homeX: number;
  homeY: number;
  lane: number;
  orbitRadius: number;
  phase: number;
  size: number;
  spin: number;
  tone: "white" | "cyan" | "amber";
  x: number;
  y: number;
};

type FieldFocus = {
  active: boolean;
  targetX: number;
  targetY: number;
  x: number;
  y: number;
};

const PARTICLE_COUNT = {
  calm: 380,
  hero: 860,
  workbench: 780,
} as const;

const SHARD_COUNT = {
  calm: 390,
  hero: 980,
  workbench: 820,
} as const;

const CORE_SHELL_POINT_COUNT = {
  calm: 380,
  hero: 560,
  workbench: 540,
} as const;

const DENSITY_BASE_AREA = 1366 * 768;

function responsiveDensityCount(
  baseCount: number,
  width: number,
  height: number,
  intensity: YucoreMotionCanvasProps["intensity"],
  layer: "particles" | "shards",
) {
  const areaScale = Math.sqrt(Math.max(1, width * height) / DENSITY_BASE_AREA);
  const cap =
    intensity === "hero"
      ? layer === "shards"
        ? 1.28
        : 1.34
      : intensity === "workbench"
        ? layer === "shards"
          ? 1.22
          : 1.3
        : 1.18;
  const floor = intensity === "hero" ? 0.94 : 0.88;

  return Math.round(baseCount * Math.min(cap, Math.max(floor, areaScale)));
}

const EARTH_LAND_SHAPES = [
  { lat: 0.52, lon: -2.18, rx: 0.36, ry: 0.2, tilt: -0.34 },
  { lat: 0.16, lon: -2.62, rx: 0.18, ry: 0.14, tilt: 0.16 },
  { lat: -0.42, lon: -1.36, rx: 0.16, ry: 0.34, tilt: 0.18 },
  { lat: 0.84, lon: -0.72, rx: 0.14, ry: 0.09, tilt: 0.08 },
  { lat: 0.46, lon: -0.06, rx: 0.26, ry: 0.14, tilt: -0.08 },
  { lat: 0.54, lon: 0.62, rx: 0.34, ry: 0.16, tilt: 0.08 },
  { lat: 0.42, lon: 1.24, rx: 0.4, ry: 0.17, tilt: -0.18 },
  { lat: -0.12, lon: 0.36, rx: 0.2, ry: 0.3, tilt: -0.06 },
  { lat: 0.02, lon: 1.5, rx: 0.2, ry: 0.13, tilt: 0.24 },
  { lat: -0.36, lon: 2.2, rx: 0.19, ry: 0.1, tilt: -0.12 },
  { lat: -0.64, lon: 2.58, rx: 0.12, ry: 0.07, tilt: 0.28 },
  { lat: 0.08, lon: -2.96, rx: 0.12, ry: 0.09, tilt: 0.36 },
] as const;

function prefersReducedMotion() {
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

function createParticles(
  count: number,
  width: number,
  height: number,
  intensity: YucoreMotionCanvasProps["intensity"],
) {
  const hero = intensity === "hero";
  const workbench = intensity === "workbench";
  const centerX = width * (hero ? 0.5 : workbench ? 0.72 : 0.56);
  const centerY = height * (hero ? 0.34 : workbench ? 0.38 : 0.44);
  const heroFieldEdge = [
    { x: 0.12, y: 0.18 },
    { x: 0.88, y: 0.24 },
    { x: 0.78, y: 0.72 },
    { x: 0.22, y: 0.76 },
    { x: 0.5, y: 0.14 },
    { x: 0.5, y: 0.84 },
    { x: 0.08, y: 0.52 },
    { x: 0.92, y: 0.56 },
    { x: 0.34, y: 0.88 },
    { x: 0.66, y: 0.12 },
    { x: 0.04, y: 0.34 },
    { x: 0.96, y: 0.76 },
  ];

  return Array.from({ length: count }, (_, index): Particle => {
    const angle = (index * 137.508 * Math.PI) / 180;
    const lane = index % (hero ? 9 : 7);
    const cluster = index % 5;
    const noise = ((index * 131) % 997) / 997;
    const radius =
      (hero ? 0.22 : 0.16) +
      Math.pow(((index * 19) % 86) / 100, 0.72) * (hero ? 0.94 : 1);
    const orbit = hero
      ? index % 20 < 5 && index % 17 !== 0
      : workbench
        ? index % 9 < 6 && index % 17 !== 0
        : index % 7 < 4 && index % 17 !== 0;
    const spreadX = width * (hero ? 0.96 : workbench ? 0.78 : 0.68);
    const spreadY = height * (hero ? 0.82 : workbench ? 0.62 : 0.54);
    const seedX = ((index * 97) % 101) / 100;
    const seedY = ((index * 53) % 103) / 100;
    const clusterX = [0.18, 0.82, 0.68, 0.34, 0.52][cluster] ?? 0.5;
    const clusterY = [0.2, 0.28, 0.68, 0.74, 0.47][cluster] ?? 0.5;
    const fieldAnchor = heroFieldEdge[index % heroFieldEdge.length];
    const homeX = orbit
      ? centerX + Math.cos(angle) * radius * spreadX
      : hero
        ? width * (fieldAnchor.x * 0.82 + seedX * 0.18)
        : width * (clusterX * 0.72 + seedX * 0.28);
    const homeY = orbit
      ? centerY + Math.sin(angle) * radius * spreadY
      : hero
        ? height * (fieldAnchor.y * 0.82 + seedY * 0.18)
        : height * (clusterY * 0.76 + seedY * 0.24);
    const tone = index % 7 === 0 ? "amber" : index % 5 === 0 ? "violet" : "cyan";

    return {
      cluster,
      energy: 0.58 + (index % 9) * 0.065,
      homeX,
      homeY,
      lane,
      noise,
      orbit,
      x: homeX + Math.cos(angle * 1.7) * 18,
      y: homeY + Math.sin(angle * 1.4) * 18,
      vx: Math.cos(angle + 1.4) * (0.22 + (index % 5) * 0.022),
      vy: Math.sin(angle + 0.7) * (0.16 + (index % 7) * 0.018),
      r: hero ? 0.98 + (index % 6) * 0.42 : 1.4 + (index % 6) * 0.62,
      phase: index * 0.37,
      spin: index % 2 === 0 ? 1 : -1,
      tone,
    };
  });
}

function createShardParticles(
  count: number,
  width: number,
  height: number,
  intensity: YucoreMotionCanvasProps["intensity"],
) {
  const hero = intensity === "hero";
  const workbench = intensity === "workbench";
  const centerX = width * (hero ? 0.5 : workbench ? 0.72 : 0.56);
  const centerY = height * (hero ? 0.35 : workbench ? 0.38 : 0.44);
  const spreadX = width * (hero ? 0.96 : workbench ? 0.62 : 0.58);
  const spreadY = height * (hero ? 0.72 : workbench ? 0.46 : 0.42);

  return Array.from({ length: count }, (_, index): ShardParticle => {
    const seedA = ((index * 41) % 113) / 113;
    const seedB = ((index * 89) % 127) / 127;
    const ring = hero
      ? 0.32 + Math.pow(((index * 29) % 101) / 100, 0.62) * 0.78
      : 0.18 + Math.pow(((index * 29) % 101) / 100, 0.62) * 0.82;
    const lane = index % 5;
    const angle =
      (index * 137.508 * Math.PI) / 180 +
      Math.sin(index * 0.74) * 0.42 +
      lane * 0.16;
    const homeX =
      centerX +
      Math.cos(angle) * spreadX * ring +
      (seedA - 0.5) * width * (hero ? 0.12 : 0.08);
    const homeY =
      centerY +
      Math.sin(angle * 1.12) * spreadY * ring +
      (seedB - 0.5) * height * (hero ? 0.18 : 0.12);
    const tone = index % 23 === 0 ? "amber" : index % 7 === 0 ? "cyan" : "white";

    return {
      angle,
      depth: 0.34 + seedB * 0.66,
      energy: 0.58 + (index % 11) * 0.052,
      homeX,
      homeY,
      lane,
      orbitRadius: ring,
      phase: index * 0.43 + seedA * Math.PI,
      size: (hero ? 2.7 : workbench ? 2.25 : 1.9) + seedA * (hero ? 6.2 : 4.4),
      spin: index % 2 === 0 ? 1 : -1,
      tone,
      x: homeX,
      y: homeY,
    };
  });
}

function drawGrid(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number,
  time: number,
) {
  const gridSize = 48;
  const offset = (time * 7) % gridSize;

  ctx.save();
  ctx.globalAlpha = 0.22;
  ctx.lineWidth = 1;
  ctx.strokeStyle = "rgba(103, 232, 249, 0.1)";

  for (let x = -gridSize + offset; x < width + gridSize; x += gridSize) {
    ctx.beginPath();
    ctx.moveTo(x, 0);
    ctx.lineTo(x, height);
    ctx.stroke();
  }

  ctx.strokeStyle = "rgba(250, 204, 21, 0.075)";
  for (let y = -gridSize + offset * 0.55; y < height + gridSize; y += gridSize) {
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(width, y);
    ctx.stroke();
  }
  ctx.restore();
}

function drawTerrain(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number,
  time: number,
  intensity: YucoreMotionCanvasProps["intensity"],
) {
  const horizon = height * 0.68;
  const vanishingX = width * 0.56;
  const rowCount = 18;
  const columnCount = 34;
  const alphaLift =
    intensity === "hero" ? 1.34 : intensity === "workbench" ? 0.86 : 1;

  ctx.save();
  ctx.lineWidth = 1;

  for (let row = 0; row < rowCount; row++) {
    const depth = row / (rowCount - 1);
    const y = horizon + Math.pow(depth, 1.72) * height * 0.42;
    const wave = Math.sin(time * 0.9 + row * 0.58) * (14 + depth * 26);

    ctx.beginPath();
    for (let column = 0; column <= columnCount; column++) {
      const ratio = column / columnCount;
      const x = ratio * width;
      const curve =
        Math.sin(ratio * Math.PI * 4.2 + time * 0.55 + row * 0.24) *
        (4 + depth * 16);
      const pointY = y + wave * Math.sin(ratio * Math.PI) + curve;

      if (column === 0) {
        ctx.moveTo(x, pointY);
      } else {
        ctx.lineTo(x, pointY);
      }
    }

    ctx.globalAlpha = (0.07 + depth * 0.3) * alphaLift;
    ctx.strokeStyle =
      row % 3 === 0
        ? "rgba(250, 204, 21, 0.36)"
        : "rgba(103, 232, 249, 0.22)";
    ctx.stroke();
  }

  for (let column = 0; column <= columnCount; column += 2) {
    const ratio = column / columnCount;
    const x = ratio * width;
    ctx.beginPath();
    ctx.moveTo(vanishingX, horizon + Math.sin(time + ratio * 8) * 8);
    ctx.lineTo(x, height + 20);
    ctx.globalAlpha = (0.11 + Math.sin(ratio * Math.PI) * 0.16) * alphaLift;
    ctx.strokeStyle = "rgba(103, 232, 249, 0.22)";
    ctx.stroke();
  }

  const laneCount = intensity === "hero" ? 9 : intensity === "workbench" ? 6 : 5;
  for (let lane = 0; lane < laneCount; lane += 1) {
    const direction = lane % 2 === 0 ? 1 : -1;
    const laneDepth = lane / Math.max(1, laneCount - 1);
    const phase = (time * (0.18 + laneDepth * 0.045) + laneDepth * 0.72) % 1;
    const startX = direction > 0 ? width * (-0.08 + phase * 0.42) : width * (1.08 - phase * 0.42);
    const endX = vanishingX + (startX - vanishingX) * 0.36;
    const startY = height * (0.94 - laneDepth * 0.22);
    const endY = horizon + laneDepth * height * 0.08 + Math.sin(time * 0.6 + lane) * 5;
    const gradient = ctx.createLinearGradient(startX, startY, endX, endY);
    const laneAlpha =
      ((intensity === "hero" ? 0.21 : 0.13) + laneDepth * (intensity === "hero" ? 0.24 : 0.18)) *
      alphaLift;

    gradient.addColorStop(0, "rgba(103, 232, 249, 0)");
    gradient.addColorStop(0.38, `rgba(103, 232, 249, ${laneAlpha})`);
    gradient.addColorStop(0.72, `rgba(250, 204, 21, ${laneAlpha * 0.82})`);
    gradient.addColorStop(1, "rgba(255, 255, 255, 0)");
    ctx.globalAlpha = 1;
    ctx.strokeStyle = gradient;
    ctx.lineWidth = intensity === "hero" ? 1.25 : 0.9;
    ctx.beginPath();
    ctx.moveTo(startX, startY);
    ctx.quadraticCurveTo(
      (startX + endX) / 2 + Math.sin(time + lane) * width * 0.02,
      (startY + endY) / 2 - height * (0.03 + laneDepth * 0.025),
      endX,
      endY,
    );
    ctx.stroke();
  }

  ctx.restore();
}

function particleColor(particle: Particle, alpha: number) {
  if (particle.tone === "amber") return `rgba(250, 204, 21, ${alpha})`;
  if (particle.tone === "violet") return `rgba(216, 180, 254, ${alpha})`;
  return `rgba(190, 242, 255, ${alpha})`;
}

function shardColor(shard: ShardParticle, alpha: number) {
  if (shard.tone === "amber") return `rgba(255, 236, 168, ${alpha})`;
  if (shard.tone === "cyan") return `rgba(204, 251, 255, ${alpha})`;
  return `rgba(255, 255, 255, ${alpha})`;
}

function clamp01(value: number) {
  return Math.max(0, Math.min(1, value));
}

function getCorePoint(
  width: number,
  height: number,
  intensity: YucoreMotionCanvasProps["intensity"],
) {
  const hero = intensity === "hero";
  const workbench = intensity === "workbench";
  return {
    x: width * (hero ? 0.5 : workbench ? 0.72 : 0.56),
    y: height * (hero ? 0.34 : workbench ? 0.38 : 0.44),
  };
}

function softBoxMask(
  x: number,
  y: number,
  centerX: number,
  centerY: number,
  halfWidth: number,
  halfHeight: number,
) {
  const dx = Math.abs(x - centerX) / Math.max(1, halfWidth);
  const dy = Math.abs(y - centerY) / Math.max(1, halfHeight);
  const boxDistance = Math.max(dx, dy);

  return clamp01((1.14 - boxDistance) / 0.58);
}

function heroReadabilityClear(
  x: number,
  y: number,
  width: number,
  height: number,
  core: ReturnType<typeof getCorePoint>,
  minSide: number,
) {
  const globeClear = clamp01(1 - Math.hypot(x - core.x, y - core.y) / (minSide * 0.46));
  const titleClear = softBoxMask(x, y, width * 0.5, height * 0.57, width * 0.42, height * 0.18);
  const statusClear = softBoxMask(x, y, width * 0.5, height * 0.45, width * 0.28, height * 0.07);
  const lowerCopyClear = softBoxMask(x, y, width * 0.5, height * 0.69, width * 0.38, height * 0.17);
  const actionClear = softBoxMask(x, y, width * 0.5, height * 0.73, width * 0.3, height * 0.1);
  const centerSpine =
    clamp01(1 - Math.abs(x - width * 0.5) / (width * 0.24)) *
    clamp01(1 - Math.abs(y - height * 0.5) / (height * 0.4)) *
    0.74;

  return Math.max(
    globeClear * 1.24,
    titleClear * 1.34,
    statusClear * 1.12,
    lowerCopyClear * 1.02,
    actionClear * 0.92,
    centerSpine,
  );
}

function heroOuterLift(
  x: number,
  y: number,
  width: number,
  height: number,
  core: ReturnType<typeof getCorePoint>,
) {
  const wing = clamp01((Math.abs(x - width * 0.5) / (width * 0.5) - 0.2) / 0.46);
  const vertical = clamp01((Math.abs(y - core.y) / (height * 0.5) - 0.14) / 0.58);
  const lowerNet = clamp01((y - height * 0.58) / (height * 0.34)) * 0.8;

  return Math.max(wing, vertical, lowerNet);
}

function workbenchReadabilityClear(
  x: number,
  y: number,
  width: number,
  height: number,
) {
  const primaryCopy = softBoxMask(
    x,
    y,
    width * 0.44,
    height * 0.36,
    width * 0.38,
    height * 0.17,
  );
  const actionArea = softBoxMask(
    x,
    y,
    width * 0.45,
    height * 0.52,
    width * 0.36,
    height * 0.12,
  );
  const lowerInput = softBoxMask(
    x,
    y,
    width * 0.5,
    height * 0.8,
    width * 0.44,
    height * 0.1,
  );
  const contentColumn = softBoxMask(
    x,
    y,
    width * 0.4,
    height * 0.54,
    width * 0.36,
    height * 0.34,
  );

  return Math.max(
    primaryCopy * 1.08,
    actionArea * 0.82,
    lowerInput * 0.78,
    contentColumn * 0.68,
  );
}

function drawOrderedOrbitLanes(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number,
  time: number,
  intensity: YucoreMotionCanvasProps["intensity"],
) {
  const hero = intensity === "hero";
  const workbench = intensity === "workbench";
  const core = getCorePoint(width, height, intensity);
  const minSide = Math.min(width, height);
  const laneCount = hero ? 12 : workbench ? 9 : 7;
  const packetCount = hero ? 8 : workbench ? 7 : 5;

  ctx.save();
  ctx.globalCompositeOperation = "screen";
  ctx.lineCap = "round";

  for (let lane = 0; lane < laneCount; lane += 1) {
    const direction = lane % 2 === 0 ? 1 : -1;
    const laneProgress = (time * 0.2 + lane / laneCount) % 1;
    const laneWave = 1 - Math.abs(laneProgress - 0.5) * 2;
    const lanePulse = 0.34 + Math.pow(Math.max(0, laneWave), 2.45) * 0.66;
    const rx = minSide * (0.19 + lane * 0.039) * (hero ? 1.16 : 1);
    const ry = rx * (0.23 + (lane % 3) * 0.04);
    const rotation = -0.46 + lane * 0.205;
    const laneAlpha = (hero ? 0.58 : workbench ? 0.36 : 0.24) * (0.68 + lanePulse * 0.5);
    const gradient = ctx.createLinearGradient(core.x - rx, core.y - ry, core.x + rx, core.y + ry);

    gradient.addColorStop(0, "rgba(103, 232, 249, 0)");
    gradient.addColorStop(0.32, `rgba(103, 232, 249, ${laneAlpha})`);
    gradient.addColorStop(0.58, `rgba(255, 255, 255, ${laneAlpha * 0.42})`);
    gradient.addColorStop(0.78, `rgba(250, 204, 21, ${laneAlpha * 0.72})`);
    gradient.addColorStop(1, "rgba(103, 232, 249, 0)");

    ctx.strokeStyle = gradient;
    ctx.lineWidth = lane % 3 === 0 ? (hero ? 1.85 : 1.55) : hero ? 1.15 : 0.95;
    ctx.shadowBlur = lane % 2 === 0 ? (hero ? 23 : 18) : hero ? 16 : 12;
    ctx.shadowColor = lane % 3 === 0 ? "rgba(250, 204, 21, 0.42)" : "rgba(103, 232, 249, 0.46)";
    ctx.setLineDash(lane % 2 === 0 ? [rx * 0.1, rx * 0.05] : [rx * 0.045, rx * 0.075]);
    ctx.lineDashOffset = time * direction * (18 + lane * 4);
    ctx.beginPath();
    ctx.ellipse(
      core.x,
      core.y,
      rx * (1.28 + lane * 0.025),
      ry,
      rotation + time * direction * (0.012 + lane * 0.0016),
      0,
      Math.PI * 2,
    );
    ctx.stroke();
    ctx.setLineDash([]);

    for (let packet = 0; packet < packetCount; packet += 1) {
      const packetPhase =
        time * direction * (0.56 + lane * 0.045) +
        (packet / packetCount) * Math.PI * 2 +
        lane * 0.52;
      const localX = Math.cos(packetPhase) * rx * (1.28 + lane * 0.025);
      const localY = Math.sin(packetPhase) * ry;
      const cos = Math.cos(rotation);
      const sin = Math.sin(rotation);
      const depth = 0.5 + Math.sin(packetPhase + lane * 0.4) * 0.5;
      const packetX = core.x + localX * cos - localY * sin;
      const packetY = core.y + localX * sin + localY * cos;
      const packetAlpha = laneAlpha * (0.62 + depth * 0.9);
      const packetSize = (hero ? 1.85 : workbench ? 1.5 : 1.3) + depth * 2.7;

      ctx.shadowBlur = 8 + depth * 12;
      ctx.shadowColor = lane % 3 === 1 ? "rgba(250, 204, 21, 0.54)" : "rgba(103, 232, 249, 0.62)";
      ctx.fillStyle =
        lane % 3 === 1
          ? `rgba(250, 204, 21, ${packetAlpha})`
          : `rgba(190, 242, 255, ${packetAlpha})`;
      ctx.beginPath();
      ctx.arc(packetX, packetY, packetSize, 0, Math.PI * 2);
      ctx.fill();
    }
  }

  ctx.restore();
}

function drawParticleNode(
  ctx: CanvasRenderingContext2D,
  particle: Particle,
  time: number,
  alpha: number,
) {
  const pulse = 0.62 + Math.sin(time * 2.6 + particle.phase) * 0.38;
  const radius = particle.r * (0.86 + pulse * 0.26);

  ctx.save();
  ctx.globalCompositeOperation = "screen";
  ctx.shadowBlur = 1.4 + particle.energy * 2.2;
  ctx.shadowColor =
    particle.tone === "amber"
      ? "rgba(250, 204, 21, 0.5)"
      : particle.tone === "violet"
        ? "rgba(216, 180, 254, 0.46)"
        : "rgba(103, 232, 249, 0.54)";
  ctx.fillStyle = particleColor(particle, alpha * 0.48);
  ctx.beginPath();
  ctx.arc(particle.x, particle.y, radius * 1.22, 0, Math.PI * 2);
  ctx.fill();

  ctx.shadowBlur = 0;
  ctx.fillStyle = particleColor(particle, Math.min(0.82, alpha * 0.96));
  ctx.beginPath();
  ctx.arc(particle.x, particle.y, radius * 0.62, 0, Math.PI * 2);
  ctx.fill();

  ctx.fillStyle = `rgba(255, 255, 255, ${Math.min(0.74, alpha * 0.58)})`;
  ctx.beginPath();
  ctx.arc(particle.x, particle.y, Math.max(0.75, radius * 0.24), 0, Math.PI * 2);
  ctx.fill();

  ctx.globalAlpha = alpha * 0.16;
  ctx.strokeStyle = "rgba(255, 255, 255, 0.78)";
  ctx.lineWidth = 0.7;
  ctx.beginPath();
  ctx.arc(particle.x, particle.y, radius * 1.58, 0, Math.PI * 2);
  ctx.stroke();
  ctx.restore();
}

function drawParticleTrail(
  ctx: CanvasRenderingContext2D,
  particle: Particle,
  time: number,
  alpha: number,
) {
  const trail = 18 + particle.energy * 26;
  const wobble = Math.sin(time * 1.8 + particle.phase) * 5;
  const tailX = particle.x - particle.vx * trail + Math.cos(particle.phase) * wobble;
  const tailY = particle.y - particle.vy * trail + Math.sin(particle.phase) * wobble;
  const gradient = ctx.createLinearGradient(tailX, tailY, particle.x, particle.y);

  gradient.addColorStop(0, "rgba(255,255,255,0)");
  gradient.addColorStop(0.55, particleColor(particle, alpha * 0.3));
  gradient.addColorStop(1, particleColor(particle, alpha * 0.95));

  ctx.save();
  ctx.lineWidth = 0.55 + particle.energy * 0.52;
  ctx.strokeStyle = gradient;
  ctx.beginPath();
  ctx.moveTo(tailX, tailY);
  ctx.quadraticCurveTo(
    (tailX + particle.x) / 2 + Math.sin(time + particle.phase) * 8,
    (tailY + particle.y) / 2 + Math.cos(time + particle.phase) * 8,
    particle.x,
    particle.y,
  );
  ctx.stroke();
  ctx.restore();
}

function drawFocusField(
  ctx: CanvasRenderingContext2D,
  focus: FieldFocus,
  width: number,
  height: number,
  time: number,
  intensity: YucoreMotionCanvasProps["intensity"],
) {
  const hero = intensity === "hero";
  const radius = Math.min(width, height) * (hero ? 0.12 : 0.09);
  const pulse = 0.5 + Math.sin(time * 2.2) * 0.5;

  ctx.save();
  const glow = ctx.createRadialGradient(
    focus.x,
    focus.y,
    0,
    focus.x,
    focus.y,
    radius * 2.35,
  );
  glow.addColorStop(0, `rgba(255,255,255,${0.05 + pulse * 0.025})`);
  glow.addColorStop(0.18, `rgba(103,232,249,${0.08 + pulse * 0.05})`);
  glow.addColorStop(0.52, `rgba(250,204,21,${0.04 + pulse * 0.025})`);
  glow.addColorStop(1, "rgba(0,0,0,0)");
  ctx.fillStyle = glow;
  ctx.fillRect(0, 0, width, height);

  ctx.globalAlpha = hero ? 0.28 : 0.16;
  ctx.strokeStyle = "rgba(190,242,255,0.72)";
  ctx.lineWidth = 1;
  for (let i = 0; i < 3; i++) {
    const ring = radius * (0.58 + i * 0.38 + pulse * 0.08);
    ctx.beginPath();
    ctx.ellipse(
      focus.x,
      focus.y,
      ring * 1.45,
      ring * 0.34,
      time * 0.35 + i * 0.72,
      0,
      Math.PI * 2,
    );
    ctx.stroke();
  }
  ctx.restore();
}

function drawEnergyBeams(
  ctx: CanvasRenderingContext2D,
  particles: Particle[],
  width: number,
  height: number,
  time: number,
  intensity: YucoreMotionCanvasProps["intensity"],
) {
  const core = getCorePoint(width, height, intensity);
  const hero = intensity === "hero";
  const stride = hero ? 7 : 10;
  const minSide = Math.min(width, height);

  ctx.save();
  ctx.lineWidth = hero ? 1.15 : 0.9;
  for (let index = 0; index < particles.length; index += stride) {
    const particle = particles[index];
    const shimmer = 0.5 + Math.sin(time * 2.1 + particle.phase) * 0.5;
    const distance = Math.hypot(particle.x - core.x, particle.y - core.y);
    const coreClear = hero ? clamp01(1 - distance / (minSide * 0.56)) : 0;
    const alpha = (hero ? 0.16 : 0.1) * particle.energy * shimmer * clamp01(1 - coreClear * 0.92);
    if (hero && alpha < 0.012) continue;

    const startRatio = hero ? 0.16 + coreClear * 0.12 : 0;
    const startX = core.x + (particle.x - core.x) * startRatio;
    const startY = core.y + (particle.y - core.y) * startRatio;
    const gradient = ctx.createLinearGradient(startX, startY, particle.x, particle.y);
    gradient.addColorStop(0, `rgba(250, 204, 21, ${alpha * 0.8})`);
    gradient.addColorStop(0.42, `rgba(103, 232, 249, ${alpha})`);
    gradient.addColorStop(1, "rgba(255, 255, 255, 0)");

    ctx.strokeStyle = gradient;
    ctx.beginPath();
    ctx.moveTo(startX, startY);
    const cx =
      (startX + particle.x) / 2 +
      Math.sin(time * 0.9 + particle.phase) * (hero ? 42 : 26);
    const cy =
      (startY + particle.y) / 2 +
      Math.cos(time * 0.7 + particle.phase) * (hero ? 30 : 18);
    ctx.quadraticCurveTo(cx, cy, particle.x, particle.y);
    ctx.stroke();
  }
  ctx.restore();
}

function drawCoreParticleShell(
  ctx: CanvasRenderingContext2D,
  centerX: number,
  centerY: number,
  radius: number,
  time: number,
  intensity: YucoreMotionCanvasProps["intensity"],
) {
  const mode = intensity ?? "calm";
  const hero = mode === "hero";
  const count = CORE_SHELL_POINT_COUNT[mode];
  const shellRadius = radius * (hero ? 1.36 : mode === "workbench" ? 1.18 : 1.08);
  const goldenAngle = Math.PI * (3 - Math.sqrt(5));
  const rotateY = time * (hero ? 0.22 : 0.16);
  const rotateX = -0.32 + Math.sin(time * 0.18) * 0.08;
  const cosX = Math.cos(rotateX);
  const sinX = Math.sin(rotateX);

  ctx.save();
  ctx.globalAlpha = 1;
  ctx.globalCompositeOperation = "screen";

  for (let index = 0; index < count; index += 1) {
    const pointY = 1 - (index / (count - 1)) * 2;
    const pointRadius = Math.sqrt(1 - pointY * pointY);
    const theta = index * goldenAngle + rotateY;
    const pointX = Math.cos(theta) * pointRadius;
    const pointZ = Math.sin(theta) * pointRadius;
    const rotatedY = pointY * cosX - pointZ * sinX;
    const rotatedZ = pointY * sinX + pointZ * cosX;
    const depth = (rotatedZ + 1) / 2;
    const shimmer = 0.72 + Math.sin(time * 1.7 + index * 0.31) * 0.28;
    const x = centerX + pointX * shellRadius * (0.9 + depth * 0.14);
    const y = centerY + rotatedY * shellRadius * (0.92 + depth * 0.06);
    const size = (hero ? 0.9 : 0.7) + depth * (hero ? 1.55 : 1.12);
    const tone = index % 17 === 0 ? "amber" : index % 7 === 0 ? "violet" : "cyan";
    const alpha = (hero ? 0.5 : 0.34) * (0.2 + depth * 0.8) * shimmer;

    ctx.shadowBlur = 3 + depth * (hero ? 7 : 5);
    ctx.shadowColor =
      tone === "amber"
        ? "rgba(250, 204, 21, 0.38)"
        : tone === "violet"
          ? "rgba(216, 180, 254, 0.34)"
          : "rgba(103, 232, 249, 0.42)";
    ctx.fillStyle =
      tone === "amber"
        ? `rgba(250, 204, 21, ${alpha})`
        : tone === "violet"
          ? `rgba(216, 180, 254, ${alpha})`
          : `rgba(190, 242, 255, ${alpha})`;
    ctx.fillRect(x - size / 2, y - size / 2, size, size);

    ctx.shadowBlur = 0;
    const coreSize = Math.max(0.7, size * 0.36);
    ctx.fillStyle = `rgba(255, 255, 255, ${Math.min(0.78, alpha * 0.62)})`;
    ctx.fillRect(x - coreSize / 2, y - coreSize / 2, coreSize, coreSize);
  }

  ctx.restore();
}

function drawProceduralEarth(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  radius: number,
  time: number,
  hero: boolean,
) {
  const rotation = time * (hero ? 0.22 : 0.16);
  const tilt = -0.24 + Math.sin(time * 0.12) * 0.05;
  const cosTilt = Math.cos(tilt);
  const sinTilt = Math.sin(tilt);

  ctx.save();
  ctx.beginPath();
  ctx.arc(x, y, radius, 0, Math.PI * 2);
  ctx.clip();

  const ocean = ctx.createRadialGradient(
    x - radius * 0.38,
    y - radius * 0.42,
    radius * 0.04,
    x + radius * 0.14,
    y + radius * 0.16,
    radius * 1.12,
  );
  ocean.addColorStop(0, "rgba(236, 254, 255, 0.5)");
  ocean.addColorStop(0.17, "rgba(95, 231, 219, 0.42)");
  ocean.addColorStop(0.42, "rgba(24, 102, 127, 0.62)");
  ocean.addColorStop(0.74, "rgba(8, 22, 35, 0.8)");
  ocean.addColorStop(1, "rgba(2, 5, 10, 0.94)");
  ctx.fillStyle = ocean;
  ctx.fillRect(x - radius, y - radius, radius * 2, radius * 2);

  ctx.globalCompositeOperation = "screen";
  ctx.globalAlpha = hero ? 0.66 : 0.52;
  ctx.strokeStyle = "rgba(190, 242, 255, 0.34)";
  ctx.lineWidth = 0.86;
  for (let i = -4; i <= 4; i += 1) {
    const latY = y + (i / 4) * radius * 0.72;
    ctx.beginPath();
    ctx.ellipse(x, latY, radius * Math.sqrt(1 - Math.min(0.86, Math.abs(i / 4))), radius * 0.08, 0, 0, Math.PI * 2);
    ctx.stroke();
  }
  for (let i = -4; i <= 4; i += 1) {
    const longitude = i / 4;
    const meridianOffset = Math.sin(rotation + longitude * Math.PI) * radius * 0.22;
    const meridianWidth = radius * (0.28 + (1 - Math.abs(longitude)) * 0.36);
    ctx.beginPath();
    ctx.ellipse(
      x + meridianOffset,
      y,
      meridianWidth,
      radius * 0.84,
      tilt * 0.65,
      0,
      Math.PI * 2,
    );
    ctx.stroke();
  }

  ctx.globalCompositeOperation = "source-over";
  for (const shape of EARTH_LAND_SHAPES) {
    const lon = shape.lon + rotation;
    const sphereX = Math.sin(lon);
    const sphereZ = Math.cos(lon);
    if (sphereZ < -0.34) continue;

    const lat = shape.lat;
    const rawY = Math.sin(lat);
    const projectedY = rawY * cosTilt - sphereZ * 0.18 * sinTilt;
    const depth = Math.max(0, (sphereZ + 0.34) / 1.34);
    const px = x + sphereX * radius * 0.78;
    const py = y + projectedY * radius * 0.78;
    const land = ctx.createRadialGradient(
      px - radius * shape.rx * 0.28,
      py - radius * shape.ry * 0.34,
      0,
      px,
      py,
      radius * Math.max(shape.rx, shape.ry) * 1.65,
    );
    land.addColorStop(0, `rgba(174, 220, 142, ${0.38 * depth})`);
    land.addColorStop(0.48, `rgba(74, 145, 104, ${0.44 * depth})`);
    land.addColorStop(1, `rgba(18, 72, 70, ${0.34 * depth})`);
    ctx.save();
    ctx.globalCompositeOperation = "source-over";
    ctx.globalAlpha = (hero ? 0.66 : 0.54) * (0.56 + depth * 0.26);
    ctx.translate(px, py);
    ctx.rotate(shape.tilt + Math.sin(time * 0.18 + shape.lon) * 0.08);
    ctx.scale(Math.max(0.16, depth * (0.58 + sphereZ * 0.14)), 1);
    ctx.fillStyle = land;
    ctx.beginPath();
    ctx.ellipse(0, 0, radius * shape.rx * 0.78, radius * shape.ry * 0.78, 0, 0, Math.PI * 2);
    ctx.fill();
    ctx.globalCompositeOperation = "screen";
    ctx.globalAlpha *= 0.54;
    ctx.beginPath();
    ctx.ellipse(
      radius * shape.rx * 0.26,
      -radius * shape.ry * 0.18,
      radius * shape.rx * 0.36,
      radius * shape.ry * 0.32,
      0.4,
      0,
      Math.PI * 2,
    );
    ctx.fill();
    ctx.globalAlpha *= 0.78;
    ctx.strokeStyle = "rgba(207, 250, 205, 0.22)";
    ctx.lineWidth = 0.7;
    ctx.beginPath();
    ctx.ellipse(0, 0, radius * shape.rx * 0.82, radius * shape.ry * 0.82, 0, 0, Math.PI * 2);
    ctx.stroke();
    ctx.restore();
  }

  ctx.save();
  ctx.globalCompositeOperation = "screen";
  ctx.globalAlpha = hero ? 0.42 : 0.3;
  ctx.strokeStyle = "rgba(255, 255, 255, 0.42)";
  ctx.lineWidth = 1.2;
  for (let i = 0; i < 5; i += 1) {
    const bandY = y + Math.sin(time * 0.16 + i * 1.38) * radius * 0.44;
    const bandX = x + (((time * 18 + i * 67) % (radius * 2.6)) - radius * 1.3);
    ctx.beginPath();
    ctx.ellipse(bandX, bandY, radius * (0.22 + i * 0.035), radius * 0.035, 0.18 + i * 0.26, 0, Math.PI * 2);
    ctx.stroke();
  }
  ctx.restore();

  const projectSurfacePoint = (lon: number, lat: number) => {
    const rotatedLon = lon + rotation;
    const sphereX = Math.sin(rotatedLon);
    const sphereZ = Math.cos(rotatedLon);
    if (sphereZ < -0.24) return null;

    const rawY = Math.sin(lat);
    const projectedY = rawY * cosTilt - sphereZ * 0.18 * sinTilt;
    const depth = Math.max(0, Math.min(1, (sphereZ + 0.24) / 1.24));

    return {
      depth,
      x: x + sphereX * radius * (0.78 + depth * 0.04),
      y: y + projectedY * radius * 0.78,
    };
  };

  ctx.save();
  ctx.globalCompositeOperation = "screen";
  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  const routeCount = hero ? 9 : 7;
  for (let route = 0; route < routeCount; route += 1) {
    const routePhase = route * 0.86 + time * (0.16 + route * 0.018);
    const baseLat = -0.38 + route * (0.76 / Math.max(1, routeCount - 1));
    const routeAlpha = (hero ? 0.62 : 0.48) * (0.76 + Math.sin(time * 0.8 + route) * 0.18);
    let openPath = false;

    ctx.beginPath();
    for (let step = 0; step <= 96; step += 1) {
      const ratio = step / 96;
      const lon = -Math.PI + ratio * Math.PI * 2;
      const lat =
        baseLat +
        Math.sin(lon * (1.25 + route * 0.08) + routePhase) * (0.08 + (route % 3) * 0.024) +
        Math.sin(lon * 2.7 - routePhase * 0.7) * 0.024;
      const projected = projectSurfacePoint(lon, lat);

      if (!projected) {
        openPath = false;
        continue;
      }

      if (!openPath) {
        ctx.moveTo(projected.x, projected.y);
        openPath = true;
      } else {
        ctx.lineTo(projected.x, projected.y);
      }
    }

    ctx.lineWidth = route % 3 === 0 ? (hero ? 1.8 : 1.38) : 1;
    ctx.shadowBlur = route % 2 === 0 ? 18 : 13;
    ctx.shadowColor =
      route % 3 === 0 ? "rgba(250, 204, 21, 0.46)" : "rgba(103, 232, 249, 0.5)";
    ctx.strokeStyle =
      route % 3 === 0
        ? `rgba(250, 204, 21, ${routeAlpha})`
        : `rgba(190, 242, 255, ${routeAlpha})`;
    ctx.stroke();

    for (let marker = 0; marker < 2; marker += 1) {
      const pulseRatio = (time * (0.12 + route * 0.018) + marker * 0.46 + route * 0.08) % 1;
      const lon = -Math.PI + pulseRatio * Math.PI * 2;
      const lat =
        baseLat +
        Math.sin(lon * (1.25 + route * 0.08) + routePhase) * (0.08 + (route % 3) * 0.024) +
        Math.sin(lon * 2.7 - routePhase * 0.7) * 0.024;
      const projected = projectSurfacePoint(lon, lat);
      if (!projected) continue;

      const markerSize = (hero ? 2.7 : 2.1) + projected.depth * 2.8;
      const markerAlpha = routeAlpha * (0.56 + projected.depth * 0.82);
      ctx.shadowBlur = 18;
      ctx.shadowColor =
        route % 3 === 0 ? "rgba(250, 204, 21, 0.72)" : "rgba(103, 232, 249, 0.82)";
      ctx.fillStyle =
        route % 3 === 0
          ? `rgba(255, 236, 168, ${markerAlpha})`
          : `rgba(236, 254, 255, ${markerAlpha})`;
      ctx.beginPath();
      ctx.arc(projected.x, projected.y, markerSize, 0, Math.PI * 2);
      ctx.fill();
    }
  }

  for (let sweep = 0; sweep < 4; sweep += 1) {
    const sweepLon = time * (0.32 + sweep * 0.06) + sweep * 2.08;
    ctx.beginPath();
    let openPath = false;
    for (let step = 0; step <= 64; step += 1) {
      const lat = -1.12 + (step / 64) * 2.24;
      const projected = projectSurfacePoint(sweepLon + Math.sin(lat * 2 + sweep) * 0.05, lat);
      if (!projected) {
        openPath = false;
        continue;
      }
      if (!openPath) {
        ctx.moveTo(projected.x, projected.y);
        openPath = true;
      } else {
        ctx.lineTo(projected.x, projected.y);
      }
    }
    ctx.lineWidth = hero ? 1.35 : 1.05;
    ctx.shadowBlur = 16;
    ctx.shadowColor = "rgba(103, 232, 249, 0.5)";
    ctx.strokeStyle =
      sweep % 2 === 0
        ? `rgba(103, 232, 249, ${hero ? 0.42 : 0.3})`
        : `rgba(250, 204, 21, ${hero ? 0.32 : 0.24})`;
    ctx.stroke();
  }
  ctx.restore();

  const shade = ctx.createLinearGradient(x - radius, y - radius, x + radius, y + radius);
  shade.addColorStop(0, "rgba(255, 255, 255, 0.16)");
  shade.addColorStop(0.42, "rgba(0, 0, 0, 0)");
  shade.addColorStop(0.72, "rgba(0, 0, 0, 0.38)");
  shade.addColorStop(1, "rgba(0, 0, 0, 0.76)");
  ctx.fillStyle = shade;
  ctx.fillRect(x - radius, y - radius, radius * 2, radius * 2);
  ctx.restore();

  ctx.save();
  ctx.globalCompositeOperation = "screen";
  ctx.strokeStyle = hero ? "rgba(190, 242, 255, 0.74)" : "rgba(190, 242, 255, 0.56)";
  ctx.lineWidth = hero ? 1.6 : 1.25;
  ctx.shadowBlur = hero ? 22 : 16;
  ctx.shadowColor = "rgba(103, 232, 249, 0.55)";
  ctx.beginPath();
  ctx.arc(x, y, radius * 1.01, 0, Math.PI * 2);
  ctx.stroke();
  ctx.restore();
}

function drawShard(
  ctx: CanvasRenderingContext2D,
  particle: Particle,
  time: number,
  alpha: number,
) {
  const size = particle.r * (2.8 + Math.sin(time * 1.7 + particle.phase) * 0.35);
  const rotation = time * 0.58 * particle.spin + particle.phase;

  ctx.save();
  ctx.translate(particle.x, particle.y);
  ctx.rotate(rotation);

  ctx.beginPath();
  ctx.moveTo(0, -size);
  ctx.lineTo(size * 0.88, size * 0.74);
  ctx.lineTo(-size * 0.72, size * 0.52);
  ctx.closePath();

  ctx.fillStyle = particleColor(particle, alpha);
  ctx.fill();

  ctx.globalAlpha = alpha * 0.75;
  ctx.strokeStyle = "rgba(255, 255, 255, 0.82)";
  ctx.stroke();

  ctx.globalAlpha = alpha * 0.38;
  ctx.translate(-1.6, 0.7);
  ctx.strokeStyle = "rgba(34, 211, 238, 0.95)";
  ctx.stroke();
  ctx.translate(3.2, -1.4);
  ctx.strokeStyle = "rgba(248, 113, 113, 0.85)";
  ctx.stroke();
  ctx.restore();
}

function drawMotionShard(
  ctx: CanvasRenderingContext2D,
  shard: ShardParticle,
  time: number,
  alpha: number,
  sizeScale = 1,
) {
  const size = shard.size * sizeScale * (0.76 + shard.depth * 0.52);
  const rotation =
    shard.angle +
    time * (0.34 + shard.energy * 0.22) * shard.spin +
    Math.sin(time * 1.3 + shard.phase) * 0.28;
  const notch = 0.55 + Math.sin(shard.phase) * 0.18;

  ctx.save();
  ctx.translate(shard.x, shard.y);
  ctx.rotate(rotation);
  ctx.globalCompositeOperation = "screen";

  ctx.beginPath();
  ctx.moveTo(0, -size);
  ctx.lineTo(size * (0.72 + notch * 0.24), size * 0.58);
  ctx.lineTo(-size * (0.5 + notch * 0.32), size * 0.48);
  ctx.closePath();

  ctx.shadowBlur = 2 + shard.depth * 5;
  ctx.shadowColor = "rgba(103, 232, 249, 0.48)";
  ctx.fillStyle = shardColor(shard, alpha * 0.82);
  ctx.fill();

  ctx.shadowBlur = 0;
  ctx.lineWidth = 0.7 + shard.depth * 0.28;
  ctx.strokeStyle = `rgba(255, 255, 255, ${alpha})`;
  ctx.stroke();

  ctx.globalAlpha = alpha * 0.58;
  ctx.translate(-1.7, 0.7);
  ctx.strokeStyle = "rgba(34, 211, 238, 0.95)";
  ctx.stroke();
  ctx.translate(3.4, -1.4);
  ctx.strokeStyle = "rgba(248, 113, 113, 0.85)";
  ctx.stroke();
  ctx.restore();
}

function drawShardField(
  ctx: CanvasRenderingContext2D,
  shards: ShardParticle[],
  width: number,
  height: number,
  time: number,
  animate: boolean,
  intensity: YucoreMotionCanvasProps["intensity"],
  motionScale = 1,
) {
  const hero = intensity === "hero";
  const workbench = intensity === "workbench";
  const core = getCorePoint(width, height, intensity);
  const baseAlpha = hero ? 0.128 : workbench ? 0.32 : 0.28;
  const minSide = Math.min(width, height);

  if (animate) {
    for (const shard of shards) {
      const layer = shard.lane % 5;
      const laneDirection = shard.lane % 2 === 0 ? 1 : -1;
      const swirl =
        shard.angle +
        time * (layer < 2 ? 0.16 : 0.08 + shard.energy * 0.055) * laneDirection +
        Math.sin(time * 0.42 + shard.phase) * (hero ? 0.26 : 0.18);
      const breathing =
        0.74 +
        Math.sin(time * (0.74 + shard.energy * 0.14) + shard.phase) *
          (hero ? 0.2 : 0.13);
      const vortexPull =
        Math.sin(time * 0.28 + shard.phase + shard.lane) *
        minSide *
        (hero ? 0.042 : 0.026);
      const irregularX =
        Math.sin(time * 1.1 + shard.phase * 1.7) * (10 + shard.depth * 22) +
        Math.cos(time * 0.47 + shard.phase) * vortexPull;
      const irregularY =
        Math.cos(time * 0.92 + shard.phase * 1.3) * (8 + shard.depth * 18) -
        Math.sin(time * 0.38 + shard.phase) * vortexPull * 0.72;
      let targetX = shard.homeX + irregularX;
      let targetY = shard.homeY + irregularY;

      if (layer < 2) {
        const tight = (hero ? 0.42 : 0.26) + shard.orbitRadius * (hero ? 0.58 : 0.34);
        targetX =
          core.x +
          Math.cos(swirl) *
            minSide *
            tight *
            (1.18 + shard.depth * 0.22) *
            breathing +
          irregularX * (hero ? 0.62 : 0.56);
        targetY =
          core.y +
          Math.sin(swirl * 1.2) *
            minSide *
            tight *
            (hero ? 0.48 + shard.depth * 0.14 : 0.42 + shard.depth * 0.1) *
            breathing +
          irregularY * (hero ? 0.6 : 0.56);
      } else if (layer < 4) {
        const flow = ((time * (0.035 + shard.energy * 0.012) * laneDirection + shard.orbitRadius) % 1 + 1) % 1;
        const bandX = -width * 0.18 + flow * width * 1.36;
        const bandSlope = laneDirection * (hero ? 0.28 : 0.2);
        const heroBandOffset = hero ? (layer === 2 ? -height * 0.18 : height * 0.38) : 0;
        const heroBandArc =
          hero
            ? Math.sin((bandX / Math.max(1, width)) * Math.PI) *
              (layer === 2 ? -height * 0.045 : height * 0.065)
            : 0;
        const bandY =
          core.y +
          heroBandOffset +
          heroBandArc +
          (bandX - width * 0.5) * bandSlope +
          Math.sin(time * 0.7 + shard.phase) * height * (hero ? 0.08 : 0.055);
        targetX = bandX + Math.sin(shard.phase + time) * (16 + shard.depth * 28);
        targetY = bandY + Math.cos(shard.phase * 1.3 + time * 0.8) * (18 + shard.depth * 34);
      } else {
        const orbitX =
          core.x +
          Math.cos(swirl) *
            width *
            (hero ? 0.56 : workbench ? 0.46 : 0.4) *
            shard.orbitRadius *
            breathing;
        const orbitY =
          core.y +
          Math.sin(swirl * 1.18) *
            height *
            (hero ? 0.42 : workbench ? 0.34 : 0.3) *
            shard.orbitRadius *
            breathing;
        targetX = orbitX * 0.55 + shard.homeX * 0.45 + irregularX * 1.35;
        targetY = orbitY * 0.52 + shard.homeY * 0.48 + irregularY * 1.35;
      }

      const response = layer < 2 ? 0.032 : layer < 4 ? 0.026 : 0.018;
      shard.x += (targetX - shard.x) * (response + shard.depth * 0.01) * motionScale;
      shard.y += (targetY - shard.y) * (response + shard.depth * 0.01) * motionScale;

      if (shard.x < -80) shard.x = width + 60;
      if (shard.x > width + 80) shard.x = -60;
      if (shard.y < -80) shard.y = height + 60;
      if (shard.y > height + 80) shard.y = -60;
    }
  }

  ctx.save();
  for (const pass of [4, 2, 3, 0, 1]) {
    for (const shard of shards) {
      if (shard.lane % 5 !== pass) continue;

      const nearCore = Math.hypot(shard.x - core.x, shard.y - core.y);
      const coreLift = Math.max(0, 1 - nearCore / (minSide * 0.52));
      const globeClear = Math.max(0, 1 - nearCore / (minSide * (hero ? 0.58 : 0.28)));
      const centerClear = hero
        ? Math.max(globeClear, heroReadabilityClear(shard.x, shard.y, width, height, core, minSide))
        : workbench
          ? Math.max(globeClear, workbenchReadabilityClear(shard.x, shard.y, width, height) * 0.9)
          : globeClear;
      const edgeLift = hero ? heroOuterLift(shard.x, shard.y, width, height, core) : 0;
      const flicker = 0.5 + Math.sin(time * 2.6 + shard.phase) * 0.5;
      const layerAlpha = pass < 2 ? 0.94 : pass < 4 ? 0.9 : 0.72;
      const alpha = Math.max(
        0,
        baseAlpha *
        layerAlpha *
        (0.42 + shard.depth * 0.7) *
        (0.7 + flicker * 0.42) *
        (0.78 + coreLift * (pass < 2 ? 0.2 : 0.12)) *
        (1 + edgeLift * 0.54) *
        clamp01(1 - centerClear * (pass < 2 ? 1.76 : pass < 4 ? 1.5 : 1.24)),
      );

      if (hero && alpha < 0.024) continue;
      if (workbench && alpha < 0.02) continue;

      drawMotionShard(
        ctx,
        shard,
        time,
        alpha,
        hero ? 1 - centerClear * 0.34 + edgeLift * 0.08 : 1,
      );
    }
  }
  ctx.restore();
}

function drawParticleField(
  ctx: CanvasRenderingContext2D,
  particles: Particle[],
  width: number,
  height: number,
  time: number,
  animate: boolean,
  intensity: YucoreMotionCanvasProps["intensity"],
  motionScale = 1,
) {
  const hero = intensity === "hero";
  const workbench = intensity === "workbench";
  const core = getCorePoint(width, height, intensity);
  const minSide = Math.min(width, height);

  if (animate) {
    for (const particle of particles) {
      if (particle.orbit) {
        const lane = particle.lane;
        const laneCount = hero ? 9 : workbench ? 7 : 6;
        const laneRadius =
          minSide *
          (hero
            ? 0.48 + lane * 0.072 + particle.noise * 0.045
            : 0.22 + lane * (workbench ? 0.038 : 0.035) + particle.noise * 0.03);
        const laneRatio = (hero ? 0.28 : 0.22) + (lane % 4) * 0.044;
        const orderedPhase = (lane % laneCount) * ((Math.PI * 2) / laneCount);
        const orbitAngle =
          orderedPhase +
          particle.phase * 0.2 +
          time * particle.spin * (0.14 + particle.energy * 0.05 + lane * 0.007) +
          Math.sin(time * 0.32 + particle.noise * 8) * 0.018;
        const laneRotation = -0.42 + lane * 0.17;
        const localX = Math.cos(orbitAngle) * laneRadius * (1.24 + lane * 0.018);
        const localY = Math.sin(orbitAngle) * laneRadius * laneRatio;
        const cos = Math.cos(laneRotation);
        const sin = Math.sin(laneRotation);
        const targetX =
          core.x +
          localX * cos -
          localY * sin +
          Math.sin(time * 1.2 + particle.phase) * (hero ? 8 : 5);
        const targetY =
          core.y +
          localX * sin +
          localY * cos +
          Math.cos(time * 1.05 + particle.phase) * (hero ? 6 : 4);

        particle.x += (targetX - particle.x) * (0.026 + particle.energy * 0.009) * motionScale;
        particle.y += (targetY - particle.y) * (0.026 + particle.energy * 0.009) * motionScale;
      } else {
        const clusterOrbit = time * (0.1 + particle.noise * 0.06) + particle.phase;
        const clusterDriftX =
          Math.cos(clusterOrbit * (1.2 + particle.cluster * 0.12)) *
          minSide *
          (0.008 + particle.noise * 0.012);
        const clusterDriftY =
          Math.sin(clusterOrbit * (0.9 + particle.cluster * 0.16)) *
          minSide *
          (0.01 + particle.noise * 0.015);
        const turbulence =
          Math.sin(time * (1.4 + particle.noise) + particle.phase * 1.7) *
          (hero ? 0.8 : 0.52);
        const driftX = particle.vx + clusterDriftX * 0.018 + turbulence;
        const driftY =
          particle.vy +
          clusterDriftY * 0.018 +
          Math.cos(time * 1.1 + particle.phase) * (hero ? 0.62 : 0.4);

        particle.x += (driftX + (particle.homeX - particle.x) * 0.0022) * motionScale;
        particle.y += (driftY + (particle.homeY - particle.y) * 0.0022) * motionScale;
      }

      if (particle.x < -40) particle.x = width + 40;
      if (particle.x > width + 40) particle.x = -40;
      if (particle.y < -40) particle.y = height + 40;
      if (particle.y > height + 40) particle.y = -40;
    }
  }

  ctx.save();
  ctx.lineWidth = 1;
  const linkRadius = hero ? 142 : workbench ? 136 : 118;
  const linkWindow = hero ? 16 : workbench ? 14 : 10;
  for (let i = 0; i < particles.length; i++) {
    const a = particles[i];
    const last = Math.min(particles.length, i + linkWindow);
    for (let j = i + 2; j < last; j += 2) {
      const b = particles[j];
      if (a.orbit !== b.orbit && (i + j) % 3 !== 0) continue;
      const dx = a.x - b.x;
      const dy = a.y - b.y;
      const distanceSquared = dx * dx + dy * dy;
      if (distanceSquared > linkRadius * linkRadius) continue;

      const distance = Math.sqrt(distanceSquared);
      const clearA = hero
        ? heroReadabilityClear(a.x, a.y, width, height, core, minSide)
        : workbench
          ? workbenchReadabilityClear(a.x, a.y, width, height)
          : 0;
      const clearB = hero
        ? heroReadabilityClear(b.x, b.y, width, height, core, minSide)
        : workbench
          ? workbenchReadabilityClear(b.x, b.y, width, height)
          : 0;
      const linkClear = Math.max(clearA, clearB);
      const linkEdgeLift = hero
        ? Math.max(
            heroOuterLift(a.x, a.y, width, height, core),
            heroOuterLift(b.x, b.y, width, height, core),
          )
        : 0;
      const alpha =
        (1 - distance / linkRadius) *
        (a.orbit && b.orbit ? 0.16 : 0.12) *
        Math.min(a.energy, b.energy) *
        clamp01(1 - linkClear * (hero ? 1.46 : workbench ? 1.08 : 0.86)) *
        (1 + linkEdgeLift * 0.36);

      if (hero && alpha < 0.004) continue;
      if (workbench && alpha < 0.006) continue;

      ctx.strokeStyle =
        a.tone === "amber" || b.tone === "amber"
          ? `rgba(250, 204, 21, ${alpha * 0.78})`
          : `rgba(103, 232, 249, ${alpha})`;
      ctx.beginPath();
      ctx.moveTo(a.x, a.y);
      ctx.lineTo(b.x, b.y);
      ctx.stroke();
    }
  }

  particles.forEach((particle, index) => {
    const pulse = 0.58 + Math.sin(time * 2 + particle.phase) * 0.42;
    const particleClear = hero
      ? heroReadabilityClear(particle.x, particle.y, width, height, core, minSide)
      : workbench
        ? workbenchReadabilityClear(particle.x, particle.y, width, height)
        : 0;
    const particleEdgeLift = hero ? heroOuterLift(particle.x, particle.y, width, height, core) : 0;
    const alpha =
      (0.2 + pulse * 0.38 * particle.energy) *
      clamp01(1 - particleClear * (hero ? (particle.orbit ? 1.68 : 1.42) : particle.orbit ? (workbench ? 0.86 : 0.86) : workbench ? 1.02 : 0.9)) *
      (1 + particleEdgeLift * (hero ? 0.48 : 0.2));

    if (hero && alpha < 0.022) return;
    if (workbench && alpha < 0.03) return;

    if (index % 3 === 0) {
      drawParticleTrail(ctx, particle, time, alpha * 0.42);
    }
    drawParticleNode(ctx, particle, time, alpha);
    if (index % 13 === 0 && particle.orbit && particle.energy > 1.02) {
      drawShard(ctx, particle, time, alpha * 0.26);
    }
  });
  ctx.restore();
}

function drawCore(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number,
  time: number,
  intensity: YucoreMotionCanvasProps["intensity"],
  coreMode: YucoreMotionCanvasProps["coreMode"] = "full",
) {
  const hero = intensity === "hero";
  const { x, y } = getCorePoint(width, height, intensity);
  const ambient = coreMode === "ambient";
  const coreScale = ambient ? 0.72 : 1;
  const radius =
    Math.min(width, height) *
    (hero ? 0.142 : intensity === "workbench" ? 0.136 : 0.122) *
    coreScale;
  const pulse = 0.5 + Math.sin(time * 1.4) * 0.5;
  const globeRadius = radius * (hero ? 1 : 0.96);

  ctx.save();
  const glow = ctx.createRadialGradient(x, y, 0, x, y, radius * 3.25);
  glow.addColorStop(0, `rgba(250, 204, 21, ${(0.08 + pulse * 0.045) * (ambient ? 0.45 : 1)})`);
  glow.addColorStop(0.22, ambient ? "rgba(34, 211, 238, 0.04)" : "rgba(34, 211, 238, 0.085)");
  glow.addColorStop(0.54, ambient ? "rgba(168, 85, 247, 0.026)" : "rgba(168, 85, 247, 0.05)");
  glow.addColorStop(1, "rgba(0, 0, 0, 0)");
  ctx.fillStyle = glow;
  ctx.fillRect(0, 0, width, height);

  if (!ambient) {
    ctx.globalAlpha = hero ? 0.36 : intensity === "workbench" ? 0.46 : 0.44;
    drawProceduralEarth(ctx, x, y, globeRadius * (0.99 + pulse * 0.018), time, hero);
    drawCoreParticleShell(ctx, x, y, radius, time, intensity);
  } else if (intensity === "workbench") {
    ctx.globalAlpha = 0.2;
    drawProceduralEarth(ctx, x, y, globeRadius * (0.98 + pulse * 0.014), time, hero);
    ctx.globalAlpha = 0.24;
    drawCoreParticleShell(ctx, x, y, radius * 0.92, time, intensity);
  }

  ctx.globalAlpha =
    (hero ? 0.58 : intensity === "workbench" ? 0.42 : 0.36) *
    (ambient ? 0.62 : 1);
  ctx.lineWidth = hero ? 1.2 : 1;
  for (let i = 0; i < 4; i++) {
    const ring = radius * (1.05 + i * 0.42 + pulse * 0.08);
    ctx.strokeStyle =
      i % 2 === 0
        ? "rgba(250, 204, 21, 0.66)"
        : "rgba(103, 232, 249, 0.58)";
    ctx.setLineDash(i % 2 === 0 ? [ring * 0.2, ring * 0.08] : [ring * 0.08, ring * 0.12]);
    ctx.lineDashOffset = time * (i % 2 === 0 ? -18 : 14);
    ctx.beginPath();
    ctx.ellipse(
      x,
      y,
      ring * 1.72,
      ring * 0.32,
      -0.28 + i * 0.16 + time * 0.04,
      0,
      Math.PI * 2,
    );
    ctx.stroke();
  }
  ctx.setLineDash([]);

  for (let i = 0; i < 3; i++) {
    const wave =
      radius *
      (1.45 + i * 0.58 + ((time * (0.22 + i * 0.05)) % 1) * 0.9);
    ctx.globalAlpha = (hero ? 0.18 : 0.1) * (1 - i * 0.18);
    ctx.strokeStyle =
      i % 2 === 0
        ? "rgba(103, 232, 249, 0.72)"
        : "rgba(250, 204, 21, 0.58)";
    ctx.beginPath();
    ctx.arc(x, y, wave, 0, Math.PI * 2);
    ctx.stroke();
  }
  ctx.restore();
}

export function YucoreMotionCanvas(props: YucoreMotionCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext("2d", {
      alpha: true,
      desynchronized: true,
    });
    if (!canvas || !ctx) return;

    let animationFrame = 0;
    let particles: Particle[] = [];
    let shards: ShardParticle[] = [];
    let width = 0;
    let height = 0;
    let lastRenderTime = Number.NEGATIVE_INFINITY;
    let focus: FieldFocus = {
      active: false,
      targetX: 0,
      targetY: 0,
      x: 0,
      y: 0,
    };
    const reduceMotion = prefersReducedMotion();
    const animate = props.active !== false;
    const motionScale = reduceMotion ? 0.22 : 1;
    const frameIntervalMs = reduceMotion ? 1000 / 18 : 1000 / 62;

    const resize = () => {
      const rect = canvas.getBoundingClientRect();
      const maxPixelRatio =
        props.intensity === "hero" ? 1.08 : props.intensity === "workbench" ? 1.12 : 1.1;
      const pixelRatio = Math.min(window.devicePixelRatio || 1, maxPixelRatio);
      width = Math.max(1, Math.floor(rect.width));
      height = Math.max(1, Math.floor(rect.height));
      canvas.width = Math.floor(width * pixelRatio);
      canvas.height = Math.floor(height * pixelRatio);
      ctx.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);
      ctx.imageSmoothingEnabled = true;
      particles = createParticles(
        responsiveDensityCount(
          PARTICLE_COUNT[props.intensity ?? "calm"],
          width,
          height,
          props.intensity ?? "calm",
          "particles",
        ),
        width,
        height,
        props.intensity ?? "calm",
      );
      shards = createShardParticles(
        responsiveDensityCount(
          SHARD_COUNT[props.intensity ?? "calm"],
          width,
          height,
          props.intensity ?? "calm",
          "shards",
        ),
        width,
        height,
        props.intensity ?? "calm",
      );
      const core = getCorePoint(width, height, props.intensity ?? "calm");
      focus = {
        active: false,
        targetX: core.x,
        targetY: core.y,
        x: core.x,
        y: core.y,
      };
    };

    const handlePointerMove = (event: PointerEvent) => {
      const rect = canvas.getBoundingClientRect();
      focus.active = true;
      focus.targetX = event.clientX - rect.left;
      focus.targetY = event.clientY - rect.top;
    };

    const handlePointerLeave = () => {
      focus.active = false;
    };

    const render = (timestamp: number) => {
      if (animate && document.hidden) {
        animationFrame = window.requestAnimationFrame(render);
        return;
      }
      if (animate && timestamp - lastRenderTime < frameIntervalMs) {
        animationFrame = window.requestAnimationFrame(render);
        return;
      }
      const frameDeltaScale =
        lastRenderTime > 0
          ? Math.min(2.4, Math.max(0.72, (timestamp - lastRenderTime) / 16.67))
          : 1;
      lastRenderTime = timestamp;
      const frameMotionScale = motionScale * frameDeltaScale;
      const time = (timestamp / 1000) * motionScale;
      if (!focus.active) {
        const core = getCorePoint(width, height, props.intensity ?? "calm");
        focus.targetX =
          core.x + Math.sin(time * 0.42) * width * (props.intensity === "hero" ? 0.08 : 0.05);
        focus.targetY =
          core.y + Math.cos(time * 0.36) * height * (props.intensity === "hero" ? 0.08 : 0.05);
      }
      focus.x += (focus.targetX - focus.x) * 0.08 * frameMotionScale;
      focus.y += (focus.targetY - focus.y) * 0.08 * frameMotionScale;

      ctx.clearRect(0, 0, width, height);
      drawCore(
        ctx,
        width,
        height,
        time,
        props.intensity ?? "calm",
        props.coreMode,
      );
      drawFocusField(ctx, focus, width, height, time, props.intensity ?? "calm");
      drawGrid(ctx, width, height, animate ? time : 0);
      drawOrderedOrbitLanes(
        ctx,
        width,
        height,
        time,
        props.intensity ?? "calm",
      );
      drawEnergyBeams(
        ctx,
        particles,
        width,
        height,
        time,
        props.intensity ?? "calm",
      );
      drawShardField(
        ctx,
        shards,
        width,
        height,
        time,
        animate,
        props.intensity ?? "calm",
        frameMotionScale,
      );
      drawParticleField(
        ctx,
        particles,
        width,
        height,
        time,
        animate,
        props.intensity ?? "calm",
        frameMotionScale,
      );
      drawTerrain(
        ctx,
        width,
        height,
        animate ? time : 0,
        props.intensity ?? "calm",
      );

      if (animate) {
        animationFrame = window.requestAnimationFrame(render);
      }
    };

    resize();
    render(0);
    window.addEventListener("resize", resize);
    if (animate && !reduceMotion) {
      window.addEventListener("pointermove", handlePointerMove, { passive: true });
      window.addEventListener("pointerleave", handlePointerLeave);
    }

    return () => {
      window.removeEventListener("resize", resize);
      if (animate && !reduceMotion) {
        window.removeEventListener("pointermove", handlePointerMove);
        window.removeEventListener("pointerleave", handlePointerLeave);
      }
      window.cancelAnimationFrame(animationFrame);
    };
  }, [props.active, props.coreMode, props.intensity]);

  return (
    <canvas
      ref={canvasRef}
      aria-hidden="true"
      className={cn("yucore-motion-canvas absolute inset-0 h-full w-full", props.className)}
    />
  );
}
