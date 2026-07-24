// SPDX-License-Identifier: MIT
//
// Pi exposes only these two terminating tools to Paper Agent. They deliberately
// perform no I/O: the owner-only oracle observes the validated arguments and
// executes the local renderer or image helper after the model turn ends.

import { defineTool, type ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { Type } from "typebox";

const Color = Type.Union([
	Type.Literal("black"), Type.Literal("gray"), Type.Literal("blue"), Type.Literal("red"),
	Type.Literal("green"), Type.Literal("yellow"), Type.Literal("cyan"), Type.Literal("magenta"),
], { description: "A color supported by the reMarkable palette" });
const StrokeWidth = Type.Union([
	Type.Literal("thin"), Type.Literal("medium"), Type.Literal("thick"),
]);
const Common = {
	color: Type.Optional(Color),
	strokeWidth: Type.Optional(StrokeWidth),
	group: Type.Optional(Type.String({ minLength: 1, maxLength: 48 })),
};
const Coordinate = (description: string) => Type.Integer({ minimum: 0, maximum: 4000, description });
const Extent = (description: string) => Type.Integer({ minimum: 1, maximum: 4000, description });
const Point = Type.Object({
	x: Coordinate("Horizontal scene coordinate"),
	y: Coordinate("Vertical scene coordinate"),
}, { additionalProperties: false });
const TextObject = Type.Object({
	type: Type.Literal("text"),
	x: Coordinate("Left edge"), y: Coordinate("Top edge"),
	width: Extent("Text box width"), height: Extent("Text box height"),
	text: Type.String({ minLength: 1, maxLength: 1200, description: "Literal text to render; use newlines only for intentional source lines" }),
	align: Type.Optional(Type.Union([Type.Literal("left"), Type.Literal("center"), Type.Literal("right")])),
	...Common,
}, { additionalProperties: false });
const LineObject = (type: "line" | "arrow") => Type.Object({
	type: Type.Literal(type),
	x1: Coordinate("Start x"), y1: Coordinate("Start y"),
	x2: Coordinate("End x"), y2: Coordinate("End y"),
	...Common,
}, { additionalProperties: false });
const RectObject = Type.Object({
	type: Type.Literal("rect"),
	x: Coordinate("Left edge"), y: Coordinate("Top edge"),
	width: Extent("Rectangle width"), height: Extent("Rectangle height"),
	radius: Type.Optional(Type.Integer({ minimum: 0, maximum: 2000 })),
	filled: Type.Optional(Type.Boolean()),
	...Common,
}, { additionalProperties: false });
const EllipseObject = Type.Object({
	type: Type.Literal("ellipse"),
	cx: Coordinate("Center x"), cy: Coordinate("Center y"),
	rx: Extent("Horizontal radius"), ry: Extent("Vertical radius"),
	filled: Type.Optional(Type.Boolean()),
	...Common,
}, { additionalProperties: false });
const CircleObject = Type.Object({
	type: Type.Literal("circle"),
	cx: Coordinate("Center x"), cy: Coordinate("Center y"),
	radius: Extent("Radius"),
	filled: Type.Optional(Type.Boolean()),
	...Common,
}, { additionalProperties: false });
const PolylineObject = Type.Object({
	type: Type.Literal("polyline"),
	points: Type.Array(Point, { minItems: 2, maxItems: 64 }),
	closed: Type.Optional(Type.Boolean()),
	filled: Type.Optional(Type.Boolean()),
	...Common,
}, { additionalProperties: false });
const GridCell = Type.Object({
	row: Type.Integer({ minimum: 0, maximum: 31 }),
	column: Type.Integer({ minimum: 0, maximum: 31 }),
	text: Type.String({ minLength: 1, maxLength: 64 }),
	color: Type.Optional(Color),
}, { additionalProperties: false });
const GridObject = Type.Object({
	type: Type.Literal("grid"),
	x: Coordinate("Left edge"), y: Coordinate("Top edge"),
	width: Extent("Grid width"), height: Extent("Grid height"),
	rows: Type.Integer({ minimum: 1, maximum: 32 }),
	columns: Type.Integer({ minimum: 1, maximum: 32 }),
	majorEvery: Type.Optional(Type.Integer({ minimum: 1, maximum: 32, description: "Draw every Nth division as a major line" })),
	majorStrokeWidth: Type.Optional(StrokeWidth),
	cells: Type.Optional(Type.Array(GridCell, { maxItems: 256 })),
	...Common,
}, { additionalProperties: false });

const SceneObject = Type.Union([
	TextObject, LineObject("line"), LineObject("arrow"), RectObject, EllipseObject,
	CircleObject, PolylineObject, GridObject,
]);

const renderScene = defineTool({
	name: "move_render_scene",
	label: "Render Scene",
	description: "Finish the turn by rendering safe semantic text, grids, tables, diagrams, charts, or vector geometry as editable native ink on the reMarkable.",
	promptSnippet: "Render a bounded semantic Scene as native notebook ink",
	promptGuidelines: [
		"Use move_render_scene as the final action for text, calculations, tables, Sudoku, calendars, mind maps, flowcharts, charts, and clean geometric drawings.",
		"Set layout to flow for ordinary prose, headings, and lists so local typography can wrap and paginate them. Set layout to spatial for tables, diagrams, charts, calendars, puzzles, and positioned labels.",
		"For a plain prose answer, use wide left-aligned text objects and add intentional line breaks so body text remains comfortably readable; do not place it in tiny label boxes.",
		"Represent repeated structures semantically: one grid object for a table or Sudoku, not dozens of unrelated line objects.",
		"Use ordinary Unicode characters for common mathematical, directional, musical, and board-game symbols; the local renderer provides a generic symbol font.",
		"Give diagram annotations generous text boxes. Long prose belongs in flow layout rather than a tiny positioned label.",
		"Use a scene canvas whose aspect ratio matches the intended output. Coordinates and object extents must remain inside that canvas.",
		"Use supported color and strokeWidth fields sparingly for hierarchy and readability. Native ink is the default.",
		"After calling move_render_scene, do not emit another assistant response in the same turn.",
	],
	parameters: Type.Object({
		version: Type.Literal(1),
		canvas: Type.Object({
			width: Type.Integer({ minimum: 48, maximum: 4000 }),
			height: Type.Integer({ minimum: 48, maximum: 4000 }),
		}, { additionalProperties: false }),
		background: Type.Optional(Type.Literal("transparent")),
		layout: Type.Optional(Type.Union([Type.Literal("flow"), Type.Literal("spatial")])),
		objects: Type.Array(SceneObject, { minItems: 1, maxItems: 192 }),
	}, { additionalProperties: false }),
	async execute(_toolCallId, params) {
		return {
			content: [{ type: "text", text: `Scene accepted (${params.objects.length} objects)` }],
			details: { version: params.version, objectCount: params.objects.length },
			terminate: true,
		};
	},
});

const generateImage = defineTool({
	name: "move_generate_image",
	label: "Generate Image",
	description: "Finish the turn by generating and inserting a native notebook image object. Use only when pixels are materially better than native ink.",
	promptSnippet: "Generate a bounded image and insert it as a notebook image object",
	promptGuidelines: [
		"Use move_generate_image only for a photo, photorealistic image, painting, watercolor, textured illustration, or poster-like art.",
		"Do not use it for text, tables, Sudoku, calendars, diagrams, charts, UI wireframes, or geometric line art; use move_render_scene for those.",
		"The prompt must be concise, self-contained English and must not request text inside the image.",
		"After calling move_generate_image, do not emit another assistant response in the same turn.",
	],
	parameters: Type.Object({
		prompt: Type.String({ minLength: 1, maxLength: 4000, description: "Concise self-contained English image prompt" }),
	}, { additionalProperties: false }),
	async execute(_toolCallId, params) {
		return {
			content: [{ type: "text", text: "Image request accepted" }],
			details: { promptLength: params.prompt.length },
			terminate: true,
		};
	},
});

export default function paperAgentTools(pi: ExtensionAPI) {
	pi.registerTool(renderScene);
	pi.registerTool(generateImage);
}
