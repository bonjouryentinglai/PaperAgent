// SPDX-License-Identifier: MIT

export const DEFAULT_SCALE_PERCENT = 100;
export const MIN_SCALE_PERCENT = 60;

/**
 * Return the largest whole-percent scale that fits the current placement.
 * A null result means even the 60% floor does not fit and the caller must
 * create a new page, reset to 100%, and lay out there.
 */
export async function chooseLargestFittingScale(fits) {
  if (typeof fits !== "function") throw new TypeError("fits must be a function");
  if (await fits(DEFAULT_SCALE_PERCENT)) return DEFAULT_SCALE_PERCENT;
  if (!(await fits(MIN_SCALE_PERCENT))) return null;

  let best = MIN_SCALE_PERCENT;
  let low = MIN_SCALE_PERCENT + 1;
  let high = DEFAULT_SCALE_PERCENT - 1;
  while (low <= high) {
    const candidate = low + Math.floor((high - low) / 2);
    if (await fits(candidate)) {
      best = candidate;
      low = candidate + 1;
    } else {
      high = candidate - 1;
    }
  }
  return best;
}

export function safePagePlacement(action, request) {
  const full = { x: 50, y: 150, width: 854, height: 1496 };
  if (action !== "beautify") return full;
  const width = Math.min(full.width, request.width);
  const height = Math.min(full.height, request.height);
  return {
    x: Math.max(full.x, Math.min(full.x + full.width - width, request.x)),
    y: full.y,
    width,
    height,
  };
}
