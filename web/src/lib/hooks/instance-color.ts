import { createStore } from "solid-js/store";

const colors = [
	"59, 130, 246", // Blue-500
	"16, 185, 129", // Green-500
	"239, 68, 68", // Red-500
	"245, 158, 11", // Yellow-500
	"99, 102, 241", // Indigo-500
	"236, 72, 153", // Pink-500
	"6, 182, 212", // Cyan-500
	"139, 92, 246", // Purple-500
	"249, 115, 22", // Orange-500
	"75, 85, 99", // Gray-500
	"34, 197, 94", // Emerald-500
	"217, 119, 6", // Amber-500
	"219, 39, 119", // Fuchsia-500
	"168, 85, 247", // Violet-500
	"225, 29, 72", // Rose-500
	"22, 163, 74", // Lime-500
	"234, 88, 12", // Warm Gray-500
	"20, 184, 166", // Teal-500
	"14, 165, 233", // Sky-500
	"82, 82, 82", // Neutral-500
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
