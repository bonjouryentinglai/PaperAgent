// SPDX-License-Identifier: MIT

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct AxisRange {
    pub minimum: i32,
    pub maximum: i32,
}

impl AxisRange {
    pub fn new(minimum: i32, maximum: i32) -> Result<Self, String> {
        if maximum <= minimum {
            return Err(format!("invalid axis range {minimum}..{maximum}"));
        }
        Ok(Self { minimum, maximum })
    }
}

/// Convert one top-left-origin canvas pixel into the digitizer's ABS range.
///
/// This maps Xochitl framebuffer pixels into the Marker digitizer range.
/// Endpoints are exact, while the half-denominator term rounds interior points instead
/// of biasing every point toward the top-left.
pub fn canvas_to_axis(pixel: i32, pixels: u32, axis: AxisRange) -> Result<i32, String> {
    if pixels < 2 {
        return Err(format!(
            "canvas axis must contain at least 2 pixels, got {pixels}"
        ));
    }
    if pixel < 0 || pixel >= pixels as i32 {
        return Err(format!("pixel {pixel} outside 0..{}", pixels - 1));
    }
    let numerator = i64::from(pixel) * i64::from(axis.maximum - axis.minimum);
    let denominator = i64::from(pixels - 1);
    Ok(axis.minimum + ((numerator + denominator / 2) / denominator) as i32)
}

pub fn canvas_point_to_axes(
    point: (i32, i32),
    canvas: (u32, u32),
    x_axis: AxisRange,
    y_axis: AxisRange,
) -> Result<(i32, i32), String> {
    Ok((
        canvas_to_axis(point.0, canvas.0, x_axis)?,
        canvas_to_axis(point.1, canvas.1, y_axis)?,
    ))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn move_xochitl_endpoints_are_exact() {
        let x = AxisRange::new(0, 6760).unwrap();
        let y = AxisRange::new(0, 11960).unwrap();
        assert_eq!(
            canvas_point_to_axes((0, 0), (954, 1696), x, y).unwrap(),
            (0, 0)
        );
        assert_eq!(
            canvas_point_to_axes((953, 1695), (954, 1696), x, y).unwrap(),
            (6760, 11960)
        );
    }

    #[test]
    fn move_midpoint_rounds_without_top_left_bias() {
        let x = AxisRange::new(0, 6760).unwrap();
        let y = AxisRange::new(0, 11960).unwrap();
        assert_eq!(
            canvas_point_to_axes((477, 848), (954, 1696), x, y).unwrap(),
            (3384, 5984)
        );
    }

    #[test]
    fn supports_non_zero_axis_minimum() {
        let axis = AxisRange::new(100, 200).unwrap();
        assert_eq!(canvas_to_axis(0, 11, axis).unwrap(), 100);
        assert_eq!(canvas_to_axis(5, 11, axis).unwrap(), 150);
        assert_eq!(canvas_to_axis(10, 11, axis).unwrap(), 200);
    }

    #[test]
    fn rejects_out_of_canvas_points() {
        let axis = AxisRange::new(0, 100).unwrap();
        assert!(canvas_to_axis(-1, 11, axis).is_err());
        assert!(canvas_to_axis(11, 11, axis).is_err());
    }
}
