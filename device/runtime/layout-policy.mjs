// SPDX-License-Identifier: MIT

export const DEFAULT_SCALE_PERCENT = 100;
export const MIN_SCALE_PERCENT = 60;

/**
 * Return the largest whole-percent scale that fits the current placement.
 * A null result means even the configured floor does not fit and the caller
 * must create a new page, reset to the configured default, and lay out there.
 */
export async function chooseLargestFittingScale(
  fits,
  {
    defaultScale = DEFAULT_SCALE_PERCENT,
    minimumScale = MIN_SCALE_PERCENT,
  } = {},
) {
  if (typeof fits !== "function") throw new TypeError("fits must be a function");
  if (!Number.isSafeInteger(defaultScale) || defaultScale < 1) {
    throw new RangeError("defaultScale must be a positive integer");
  }
  if (!Number.isSafeInteger(minimumScale)
      || minimumScale < 1
      || minimumScale > defaultScale) {
    throw new RangeError("minimumScale must be between 1 and defaultScale");
  }
  if (await fits(defaultScale)) return defaultScale;
  if (!(await fits(minimumScale))) return null;

  let best = minimumScale;
  let low = minimumScale + 1;
  let high = defaultScale - 1;
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
