// SPDX-License-Identifier: MIT
//
// Hover/down/move/up ordering adapted from smart_remarkable's `Pen` at
// cb787065281b7211b012bd5e5d9be751fe5adaef. Keeping the protocol pure makes
// it testable without a tablet or writable input device.

pub const EV_SYN: u16 = 0;
pub const EV_KEY: u16 = 1;
pub const EV_ABS: u16 = 3;
pub const SYN_REPORT: u16 = 0;
pub const ABS_X: u16 = 0;
pub const ABS_Y: u16 = 1;
pub const ABS_PRESSURE: u16 = 24;
pub const ABS_DISTANCE: u16 = 25;
pub const BTN_TOOL_PEN: u16 = 320;
pub const BTN_TOUCH: u16 = 330;

pub const DRAW_PRESSURE: i32 = 2100;
pub const HOVER_DISTANCE: i32 = 100;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Event {
    pub event_type: u16,
    pub code: u16,
    pub value: i32,
}

impl Event {
    const fn new(event_type: u16, code: u16, value: i32) -> Self {
        Self {
            event_type,
            code,
            value,
        }
    }
}

pub fn hover_at(x: i32, y: i32) -> Vec<Event> {
    vec![
        Event::new(EV_ABS, ABS_X, x),
        Event::new(EV_ABS, ABS_Y, y),
        Event::new(EV_KEY, BTN_TOOL_PEN, 1),
        Event::new(EV_KEY, BTN_TOUCH, 0),
        Event::new(EV_ABS, ABS_PRESSURE, 0),
        Event::new(EV_ABS, ABS_DISTANCE, HOVER_DISTANCE),
        Event::new(EV_SYN, SYN_REPORT, 0),
    ]
}

pub fn touch_down() -> Vec<Event> {
    vec![
        Event::new(EV_KEY, BTN_TOUCH, 1),
        Event::new(EV_ABS, ABS_PRESSURE, DRAW_PRESSURE),
        Event::new(EV_ABS, ABS_DISTANCE, 0),
        Event::new(EV_SYN, SYN_REPORT, 0),
    ]
}

pub fn move_to(x: i32, y: i32) -> Vec<Event> {
    vec![
        Event::new(EV_ABS, ABS_X, x),
        Event::new(EV_ABS, ABS_Y, y),
        Event::new(EV_SYN, SYN_REPORT, 0),
    ]
}

pub fn pen_up() -> Vec<Event> {
    vec![
        Event::new(EV_ABS, ABS_PRESSURE, 0),
        Event::new(EV_ABS, ABS_DISTANCE, HOVER_DISTANCE),
        Event::new(EV_KEY, BTN_TOUCH, 0),
        Event::new(EV_KEY, BTN_TOOL_PEN, 0),
        Event::new(EV_SYN, SYN_REPORT, 0),
    ]
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn hover_positions_before_touching() {
        let events = hover_at(10, 20);
        assert_eq!(events[0], Event::new(EV_ABS, ABS_X, 10));
        assert_eq!(events[1], Event::new(EV_ABS, ABS_Y, 20));
        assert!(events.contains(&Event::new(EV_KEY, BTN_TOUCH, 0)));
        assert_eq!(events.last(), Some(&Event::new(EV_SYN, SYN_REPORT, 0)));
    }

    #[test]
    fn down_sets_pressure_only_after_hover() {
        let events = touch_down();
        assert_eq!(DRAW_PRESSURE, 2100);
        assert_eq!(events[0], Event::new(EV_KEY, BTN_TOUCH, 1));
        assert!(events.contains(&Event::new(EV_ABS, ABS_PRESSURE, DRAW_PRESSURE)));
        assert!(events.contains(&Event::new(EV_ABS, ABS_DISTANCE, 0)));
    }

    #[test]
    fn up_releases_touch_and_tool() {
        let events = pen_up();
        assert!(events.contains(&Event::new(EV_KEY, BTN_TOUCH, 0)));
        assert!(events.contains(&Event::new(EV_KEY, BTN_TOOL_PEN, 0)));
        assert_eq!(events.last(), Some(&Event::new(EV_SYN, SYN_REPORT, 0)));
    }
}
