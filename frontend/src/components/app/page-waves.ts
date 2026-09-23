/** Wave transition shared by the landing CTA (rise) and the platform (exit). Blue → white → black; black ends on top. */
export const WAVE_COLORS = ["#2f6ad1", "#ffffff", "#0e1512"] as const;
const PATH = "M0,70 C180,10 360,130 540,70 C720,10 900,130 1080,70 C1260,10 1440,130 1620,70 L1620,140 L0,140 Z";

export function buildWaveStack(state: "start" | "covering"): HTMLDivElement {
  const stack = document.createElement("div");
  stack.className = "wave-stack";
  WAVE_COLORS.forEach((color, i) => {
    const w = document.createElement("div");
    w.className = "wave" + (state === "covering" ? " in" : "");
    w.style.zIndex = String(i + 1);
    w.style.transitionDelay = `${i * 110}ms`;
    w.innerHTML = `<svg viewBox="0 0 1620 140" preserveAspectRatio="none"><path d="${PATH}" fill="${color}"/></svg><div class="wave-body" style="background:${color}"></div>`;
    stack.appendChild(w);
  });
  document.body.appendChild(stack);
  return stack;
}
