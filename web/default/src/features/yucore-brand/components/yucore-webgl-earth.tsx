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

interface YucoreWebglEarthProps {
  active?: boolean;
  className?: string;
  density?: "loader" | "persistent";
  timeOffsetSeconds?: number;
}

const vertexShaderSource = `
attribute vec2 a_position;
varying vec2 v_uv;

void main() {
  v_uv = a_position * 0.5 + 0.5;
  gl_Position = vec4(a_position, 0.0, 1.0);
}
`;

const fragmentShaderSource = `
precision highp float;

varying vec2 v_uv;
uniform vec2 u_resolution;
uniform float u_time;
uniform float u_loader;

float hash(vec2 p) {
  return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453123);
}

float noise(vec2 p) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  vec2 u = f * f * (3.0 - 2.0 * f);
  return mix(
    mix(hash(i + vec2(0.0, 0.0)), hash(i + vec2(1.0, 0.0)), u.x),
    mix(hash(i + vec2(0.0, 1.0)), hash(i + vec2(1.0, 1.0)), u.x),
    u.y
  );
}

float fbm(vec2 p) {
  float value = 0.0;
  float amplitude = 0.52;
  for (int i = 0; i < 4; i++) {
    value += noise(p) * amplitude;
    p = p * 2.05 + vec2(13.2, 7.7);
    amplitude *= 0.5;
  }
  return value;
}

float lonDelta(float a, float b) {
  return atan(sin(a - b), cos(a - b));
}

float ellipseBlob(
  float lon,
  float lat,
  float centerLon,
  float centerLat,
  float radiusLon,
  float radiusLat,
  float tilt
) {
  float dx = lonDelta(lon, centerLon);
  float dy = lat - centerLat;
  float c = cos(tilt);
  float s = sin(tilt);
  vec2 q = vec2(c * dx + s * dy, -s * dx + c * dy) / vec2(radiusLon, radiusLat);
  return 1.0 - dot(q, q);
}

float earthLandShape(float lon, float lat, vec2 mapUv) {
  float shape = -2.0;
  shape = max(shape, ellipseBlob(lon, lat, -2.18, 0.68, 0.62, 0.34, -0.38)); // North America
  shape = max(shape, ellipseBlob(lon, lat, -1.35, -0.45, 0.26, 0.56, 0.18)); // South America
  shape = max(shape, ellipseBlob(lon, lat, -0.72, 1.12, 0.23, 0.16, 0.08)); // Greenland
  shape = max(shape, ellipseBlob(lon, lat, 0.42, 0.62, 0.64, 0.27, 0.08)); // Europe / West Asia
  shape = max(shape, ellipseBlob(lon, lat, 1.32, 0.56, 0.82, 0.3, -0.16)); // Asia
  shape = max(shape, ellipseBlob(lon, lat, 0.42, -0.18, 0.34, 0.5, -0.08)); // Africa
  shape = max(shape, ellipseBlob(lon, lat, 1.5, 0.02, 0.34, 0.22, 0.2)); // Southeast Asia
  shape = max(shape, ellipseBlob(lon, lat, 2.28, -0.55, 0.32, 0.17, -0.12)); // Australia
  shape = max(shape, ellipseBlob(lon, lat, -2.85, 0.1, 0.22, 0.18, 0.36)); // Pacific islands arc
  shape = max(shape, smoothstep(1.18, 1.44, abs(lat)) * 0.34 - 0.06);

  float coastline = fbm(mapUv * vec2(24.0, 12.0) + vec2(5.7, 2.8));
  float ridges = sin(lon * 5.0 + fbm(mapUv * 8.0) * 1.8) * 0.035;
  return shape + (coastline - 0.5) * 0.22 + ridges;
}

float lineMask(float value, float width) {
  return 1.0 - smoothstep(0.0, width, abs(value));
}

float routeLine(float lon, float lat, float phase, float bias, float amp, float width) {
  float path =
    bias +
    sin(lon * 1.55 + phase) * amp +
    sin(lon * 3.15 - phase * 0.72) * amp * 0.34;
  return lineMask(lat - path, width);
}

mat3 rotateY(float angle) {
  float c = cos(angle);
  float s = sin(angle);
  return mat3(c, 0.0, -s, 0.0, 1.0, 0.0, s, 0.0, c);
}

mat3 rotateX(float angle) {
  float c = cos(angle);
  float s = sin(angle);
  return mat3(1.0, 0.0, 0.0, 0.0, c, -s, 0.0, s, c);
}

void main() {
  vec2 uv = v_uv * 2.0 - 1.0;
  uv.x *= u_resolution.x / max(u_resolution.y, 1.0);
  float r2 = dot(uv, uv);
  if (r2 > 1.0) {
    discard;
  }

  float z = sqrt(1.0 - r2);
  vec3 normal = normalize(vec3(uv.x, -uv.y, z));
  vec3 world = rotateY(u_time * mix(0.22, 0.34, u_loader)) * rotateX(-0.24) * normal;
  vec3 light = normalize(vec3(-0.36, 0.48, 0.8));
  float diffuse = clamp(dot(normal, light) * 0.5 + 0.5, 0.0, 1.0);
  float rim = pow(1.0 - z, 2.4);

  float lon = atan(world.z, world.x);
  float lat = asin(clamp(world.y, -1.0, 1.0));
  vec2 mapUv = vec2(lon / 6.2831853 + 0.5, lat / 3.1415926 + 0.5);

  float continent =
    fbm(mapUv * vec2(5.8, 3.4) + vec2(0.18, 0.0)) * 0.62 +
    fbm(mapUv * vec2(14.0, 7.5) + vec2(1.2, -0.4)) * 0.28 +
    sin(lon * 2.2 + sin(lat * 4.0)) * 0.07;
  float shapedContinent = earthLandShape(lon, lat, mapUv);
  float mapLand = smoothstep(0.52, 0.7, continent + abs(lat) * 0.03);
  float shapeLand = smoothstep(-0.04, 0.38, shapedContinent);
  float ice = smoothstep(1.1, 1.38, abs(lat) + fbm(mapUv * vec2(18.0, 8.0)) * 0.12);
  float land = clamp(max(shapeLand * 0.86, mapLand * 0.48), 0.0, 1.0);
  float coast =
    (smoothstep(-0.08, 0.08, shapedContinent) - smoothstep(0.36, 0.58, shapedContinent)) +
    (smoothstep(0.48, 0.58, continent) - smoothstep(0.62, 0.72, continent)) * 0.42;
  coast = clamp(coast, 0.0, 1.0);
  float terrain =
    smoothstep(0.62, 0.86, fbm(mapUv * vec2(34.0, 16.0) + vec2(7.1, 2.4))) *
    land *
    (0.35 + diffuse * 0.45);
  float cloud =
    smoothstep(
      0.56,
      0.78,
      fbm(mapUv * vec2(18.0, 9.0) + vec2(u_time * 0.055, u_time * 0.018))
    ) * (0.38 + 0.32 * sin((lat + lon) * 3.0 + u_time * 0.3));
  float gridLat = 1.0 - smoothstep(0.0, 0.014, abs(fract((mapUv.y + 0.005) * 10.0) - 0.5));
  float gridLon = 1.0 - smoothstep(0.0, 0.012, abs(fract(mapUv.x * 16.0) - 0.5));
  float fineGridLon = 1.0 - smoothstep(0.0, 0.008, abs(fract(mapUv.x * 32.0 + u_time * 0.018) - 0.5));
  float grid = max(max(gridLat, gridLon), fineGridLon * 0.55) * 0.32;
  float equator = lineMask(lat + sin(lon * 2.0 + u_time * 0.42) * 0.012, 0.022);
  float meridianSweep =
    lineMask(lonDelta(lon, u_time * 0.34), 0.022) +
    lineMask(lonDelta(lon, -u_time * 0.24 + 2.18), 0.018);
  float routeA = routeLine(lon, lat, u_time * 0.42, -0.2, 0.12, 0.026);
  float routeB = routeLine(lon, lat, -u_time * 0.34 + 1.7, 0.08, 0.1, 0.022);
  float routeC = routeLine(lon, lat, u_time * 0.26 + 3.1, 0.34, 0.08, 0.02);
  float routeD = routeLine(lon, lat, -u_time * 0.5 + 4.2, -0.42, 0.06, 0.018);
  float route = max(max(routeA, routeB), max(routeC, routeD));
  float routePulse =
    smoothstep(0.82, 1.0, 0.5 + 0.5 * sin(lon * 5.2 - u_time * 3.4)) * routeA +
    smoothstep(0.86, 1.0, 0.5 + 0.5 * sin(lon * 4.4 + u_time * 3.0 + 1.4)) * routeB +
    smoothstep(0.88, 1.0, 0.5 + 0.5 * sin(lon * 6.0 - u_time * 2.7 + 2.8)) * routeC +
    smoothstep(0.86, 1.0, 0.5 + 0.5 * sin(lon * 7.2 + u_time * 3.8 + 0.8)) * routeD;
  float routeVisibility = smoothstep(-0.12, 0.58, z);

  vec3 ocean = mix(vec3(0.004, 0.028, 0.062), vec3(0.018, 0.32, 0.4), diffuse);
  vec3 landColor = mix(vec3(0.034, 0.18, 0.12), vec3(0.22, 0.54, 0.34), diffuse);
  vec3 coastColor = vec3(0.86, 0.82, 0.48) * coast * 0.3;
  vec3 color = mix(ocean, landColor, land);
  color += coastColor;
  color += vec3(0.48, 0.76, 0.5) * terrain * 0.22;
  color = mix(color, vec3(0.82, 0.95, 0.9), ice * 0.52);
  color += vec3(0.62, 1.0, 0.96) * cloud * 0.24;
  color += vec3(0.52, 0.9, 1.0) * grid * (0.58 + routeVisibility * 0.42);
  color += vec3(0.58, 1.0, 0.96) * route * routeVisibility * (0.7 + diffuse * 0.54);
  color += vec3(1.0, 0.78, 0.28) * routePulse * routeVisibility * 1.18;
  color += vec3(0.5, 0.92, 1.0) * meridianSweep * routeVisibility * 0.72;
  color += vec3(1.0, 0.84, 0.34) * equator * routeVisibility * 0.62;
  color += vec3(0.3, 0.88, 1.0) * rim * 0.72;
  color += vec3(1.0, 0.78, 0.24) * pow(max(0.0, dot(normal, vec3(0.2, -0.1, 0.98))), 8.0) * 0.28;

  float terminator = smoothstep(-0.2, 0.8, dot(normal, light));
  float nightLights =
    smoothstep(0.68, 0.94, fbm(mapUv * vec2(58.0, 28.0) + vec2(3.4, 8.8))) *
    land *
    (1.0 - terminator) *
    (0.22 + coast * 0.8);
  color += vec3(1.0, 0.72, 0.24) * nightLights * 0.58;
  color *= 0.42 + terminator * 0.76;
  color = pow(color, vec3(0.92));

  float alpha = (1.0 - smoothstep(0.88, 1.0, r2)) * (0.96 + rim * 0.24);
  gl_FragColor = vec4(color, alpha);
}
`;

function prefersReducedMotion() {
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

function createShader(gl: WebGLRenderingContext, type: number, source: string) {
  const shader = gl.createShader(type);
  if (!shader) return null;
  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    gl.deleteShader(shader);
    return null;
  }
  return shader;
}

function createProgram(gl: WebGLRenderingContext) {
  const vertexShader = createShader(gl, gl.VERTEX_SHADER, vertexShaderSource);
  const fragmentShader = createShader(gl, gl.FRAGMENT_SHADER, fragmentShaderSource);
  if (!vertexShader || !fragmentShader) return null;

  const program = gl.createProgram();
  if (!program) return null;
  gl.attachShader(program, vertexShader);
  gl.attachShader(program, fragmentShader);
  gl.linkProgram(program);

  gl.deleteShader(vertexShader);
  gl.deleteShader(fragmentShader);

  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
    gl.deleteProgram(program);
    return null;
  }

  return program;
}

export function YucoreWebglEarth(props: YucoreWebglEarthProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const gl = canvas?.getContext("webgl", {
      alpha: true,
      antialias: false,
      depth: false,
      premultipliedAlpha: true,
      powerPreference: "high-performance",
    });
    if (!canvas || !gl) return;

    const program = createProgram(gl);
    if (!program) return;

    const positionLocation = gl.getAttribLocation(program, "a_position");
    const resolutionLocation = gl.getUniformLocation(program, "u_resolution");
    const timeLocation = gl.getUniformLocation(program, "u_time");
    const loaderLocation = gl.getUniformLocation(program, "u_loader");
    const buffer = gl.createBuffer();
    if (!buffer) return;

    gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
    gl.bufferData(
      gl.ARRAY_BUFFER,
      new Float32Array([-1, -1, 1, -1, -1, 1, -1, 1, 1, -1, 1, 1]),
      gl.STATIC_DRAW,
    );
    gl.enable(gl.BLEND);
    gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);

    let width = 0;
    let height = 0;
    let animationFrame = 0;
    let lastRenderTime = Number.NEGATIVE_INFINITY;
    const reduceMotion = prefersReducedMotion();
    const animate = props.active !== false;
    const motionScale = reduceMotion ? 0.2 : 1;
    const frameIntervalMs = reduceMotion ? 1000 / 18 : 1000 / 62;
    const timeOffsetSeconds = props.timeOffsetSeconds ?? 0;

    const resize = () => {
      const rect = canvas.getBoundingClientRect();
      const maxPixelRatio = props.density === "loader" ? 1.08 : 1.15;
      const pixelRatio = Math.min(window.devicePixelRatio || 1, maxPixelRatio);
      width = Math.max(1, Math.floor(rect.width));
      height = Math.max(1, Math.floor(rect.height));
      canvas.width = Math.floor(width * pixelRatio);
      canvas.height = Math.floor(height * pixelRatio);
      gl.viewport(0, 0, canvas.width, canvas.height);
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
      lastRenderTime = timestamp;

      gl.clearColor(0, 0, 0, 0);
      gl.clear(gl.COLOR_BUFFER_BIT);
      gl.useProgram(program);
      gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
      gl.enableVertexAttribArray(positionLocation);
      gl.vertexAttribPointer(positionLocation, 2, gl.FLOAT, false, 0, 0);
      gl.uniform2f(resolutionLocation, width, height);
      gl.uniform1f(timeLocation, (timestamp / 1000 + timeOffsetSeconds) * motionScale);
      gl.uniform1f(loaderLocation, props.density === "loader" ? 1 : 0);
      gl.drawArrays(gl.TRIANGLES, 0, 6);

      if (animate) {
        animationFrame = window.requestAnimationFrame(render);
      }
    };

    resize();
    render(0);
    window.addEventListener("resize", resize);

    return () => {
      window.cancelAnimationFrame(animationFrame);
      window.removeEventListener("resize", resize);
      gl.deleteBuffer(buffer);
      gl.deleteProgram(program);
    };
  }, [props.active, props.density, props.timeOffsetSeconds]);

  return (
    <canvas
      ref={canvasRef}
      aria-hidden="true"
      className={cn("yucore-webgl-earth absolute inset-0 h-full w-full rounded-full", props.className)}
    />
  );
}
