import { createStore } from "solid-js/store";

const colors = [
	"#3B82F6", // Blue-500
	"#10B981", // Green-500
	"#EF4444", // Red-500
	"#F59E0B", // Yellow-500
	"#6366F1", // Indigo-500
	"#EC4899", // Pink-500
	"#06B6D4", // Cyan-500
	"#8B5CF6", // Purple-500
	"#F97316", // Orange-500
	"#4B5563", // Gray-500
	"#22C55E", // Emerald-500
	"#D97706", // Amber-500
	"#DB2777", // Fuchsia-500
	"#A855F7", // Violet-500
	"#E11D48", // Rose-500
	"#16A34A", // Lime-500
	"#EA580C", // Warm Gray-500
	"#14B8A6", // Teal-500
	"#0EA5E9", // Sky-500
	"#525252", // Neutral-500
];

const usedColors = new Set<string>();

const randomColorForInstance = (instance: string) => {
	if (!usedColors.has(instance)) {
		usedColors.add(instance);
		return colors[usedColors.size - 1];
	}
	return colors[usedColors.size % colors.length];
};

export const instancesColorMap = createStore<{ [key: string]: string }>({});
export const useInstanceColor = (instance: string) => {
	const [state, setState] = instancesColorMap;
	if (!state[instance]) {
		const randomColor = randomColorForInstance(instance);
		setState(instance, randomColor);
	}
	return state[instance];
};
