import { expect, describe, it } from "vitest";
import { SortedList } from "@/lib/store/sorted-list";

type ListItem = {
	id: string;
	number: number;
};

describe("Sorted list", () => {
	it("Sort on insert and return correct item at index", () => {
		const list = new SortedList<ListItem>((a, b) => a.number - b.number);
		list.insert({ id: "5", number: 5 });
		list.insert({ id: "1", number: 1 });
		list.insert({ id: "3", number: 3 });
		list.insert({ id: "2", number: 2 });
		list.insert({ id: "4", number: 4 });

		expect(list.get(0)).toStrictEqual({ id: "1", number: 1 });
		expect(list.get(1)).toStrictEqual({ id: "2", number: 2 });
		expect(list.get(2)).toStrictEqual({ id: "3", number: 3 });
		expect(list.get(3)).toStrictEqual({ id: "4", number: 4 });
		expect(list.get(4)).toStrictEqual({ id: "5", number: 5 });
	});

	it("Reverse sort on insert and return correct item at index", () => {
		const list = new SortedList<ListItem>((a, b) => b.number - a.number);
		list.insert({ id: "5", number: 5 });
		list.insert({ id: "1", number: 1 });
		list.insert({ id: "3", number: 3 });
		list.insert({ id: "2", number: 2 });
		list.insert({ id: "4", number: 4 });

		expect(list.get(0)).toStrictEqual({ id: "5", number: 5 });
		expect(list.get(1)).toStrictEqual({ id: "4", number: 4 });
		expect(list.get(2)).toStrictEqual({ id: "3", number: 3 });
		expect(list.get(3)).toStrictEqual({ id: "2", number: 2 });
		expect(list.get(4)).toStrictEqual({ id: "1", number: 1 });
	});

	it("Return correct size", () => {
		const list = new SortedList<ListItem>((a, b) => a.number - b.number);
		list.insert({ id: "5", number: 5 });
		list.insert({ id: "1", number: 1 });
		list.insert({ id: "3", number: 3 });
		list.insert({ id: "2", number: 2 });
		list.insert({ id: "4", number: 4 });

		expect(list.size).toBe(5);
	});

	it("Has correct head and tail", () => {
		const list = new SortedList<ListItem>((a, b) => a.number - b.number);
		list.insert({ id: "5", number: 5 });
		list.insert({ id: "1", number: 1 });
		list.insert({ id: "3", number: 3 });
		list.insert({ id: "2", number: 2 });
		list.insert({ id: "4", number: 4 });

		expect(list.getHead()?.value).toStrictEqual({ id: "1", number: 1 });
		expect(list.getTail()?.value).toStrictEqual({ id: "5", number: 5 });
	});

	it("Has correct head and tail with reverse sort", () => {
		const list = new SortedList<ListItem>((a, b) => b.number - a.number);
		list.insert({ id: "5", number: 5 });
		list.insert({ id: "1", number: 1 });
		list.insert({ id: "3", number: 3 });
		list.insert({ id: "2", number: 2 });
		list.insert({ id: "4", number: 4 });

		expect(list.getHead()?.value).toStrictEqual({ id: "5", number: 5 });
		expect(list.getTail()?.value).toStrictEqual({ id: "1", number: 1 });
	});

	it("Has correct head and tail with multiple input lines", () => {
		const list = new SortedList<ListItem>((a, b) => b.number - a.number);
		list.insert({ id: "5", number: 5 });

		list.insertMany([
			{ id: "1", number: 1 },
			{ id: "3", number: 3 },
			{ id: "2", number: 2 },
			{ id: "4", number: 4 },
		]);

		expect(list.getHead()?.value).toStrictEqual({ id: "5", number: 5 });
		expect(list.getTail()?.value).toStrictEqual({ id: "1", number: 1 });
	});

	it("Has correct items after insert many", () => {
		const list = new SortedList<ListItem>((a, b) => a.number - b.number);
		list.insert({ id: "5", number: 5 });
		list.insert({ id: "1", number: 1 });
		list.insert({ id: "3", number: 3 });
		list.insert({ id: "2", number: 2 });
		list.insert({ id: "4", number: 4 });

		list.insertMany([
			{ id: "6", number: 6 },
			{ id: "7", number: 7 },
			{ id: "8", number: 8 },
		]);

		expect(list.get(0)).toStrictEqual({ id: "1", number: 1 });
		expect(list.get(1)).toStrictEqual({ id: "2", number: 2 });
		expect(list.get(2)).toStrictEqual({ id: "3", number: 3 });
		expect(list.get(3)).toStrictEqual({ id: "4", number: 4 });
		expect(list.get(4)).toStrictEqual({ id: "5", number: 5 });
		expect(list.get(5)).toStrictEqual({ id: "6", number: 6 });
		expect(list.get(6)).toStrictEqual({ id: "7", number: 7 });
		expect(list.get(7)).toStrictEqual({ id: "8", number: 8 });
	});

	it("Has correct items after prepended many", () => {
		const list = new SortedList<ListItem>((a, b) => a.number - b.number);
		list.insert({ id: "10", number: 10 });
		list.insert({ id: "11", number: 11 });
		list.insert({ id: "12", number: 12 });
		list.insert({ id: "13", number: 13 });
		list.insert({ id: "14", number: 14 });

		list.insertMany([
			{ id: "6", number: 6 },
			{ id: "7", number: 7 },
			{ id: "8", number: 8 },
		]);

		expect(list.get(0)).toStrictEqual({ id: "6", number: 6 });
		expect(list.get(1)).toStrictEqual({ id: "7", number: 7 });
		expect(list.get(2)).toStrictEqual({ id: "8", number: 8 });
		expect(list.get(3)).toStrictEqual({ id: "10", number: 10 });
		expect(list.get(4)).toStrictEqual({ id: "11", number: 11 });
		expect(list.get(5)).toStrictEqual({ id: "12", number: 12 });
		expect(list.get(6)).toStrictEqual({ id: "13", number: 13 });
		expect(list.get(7)).toStrictEqual({ id: "14", number: 14 });
	});

	it("Head and tail are equal if only one element", () => {
		const list = new SortedList<ListItem>((a, b) => a.number - b.number);
		list.insert({ id: "5", number: 5 });

		expect(list.getHead()).toStrictEqual(list.getHead());
	});

	it("Head and tail null if empty list", () => {
		const list = new SortedList<ListItem>((a, b) => a.number - b.number);

		expect(list.size).toBe(0);
		expect(list.getHead()).toBeNull();
		expect(list.getHead()).toBeNull();
	});

	it("Does not insert duplicate items", () => {
		const list = new SortedList<ListItem>((a, b) => a.number - b.number);
		list.insert({ id: "5", number: 5 });
		list.insert({ id: "5", number: 5 });

		expect(list.size).toBe(1);
	});
});
