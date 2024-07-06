import { expect, describe, it } from "vitest";
import { SortedList } from "~/lib/store/sorted-list";

describe("Sorted list", () => {
	it("Sort on insert and return correct item at index", () => {
		const list = new SortedList<number>((a, b) => a - b);
		list.insert(5);
		list.insert(1);
		list.insert(3);
		list.insert(2);
		list.insert(4);

		expect(list.get(0)).toBe(1);
		expect(list.get(1)).toBe(2);
		expect(list.get(2)).toBe(3);
		expect(list.get(3)).toBe(4);
		expect(list.get(4)).toBe(5);
	});

	it("Reverse sort on insert and return correct item at index", () => {
		const list = new SortedList<number>((a, b) => b - a);
		list.insert(5);
		list.insert(1);
		list.insert(3);
		list.insert(2);
		list.insert(4);

		expect(list.get(0)).toBe(5);
		expect(list.get(1)).toBe(4);
		expect(list.get(2)).toBe(3);
		expect(list.get(3)).toBe(2);
		expect(list.get(4)).toBe(1);
	});

	it("Return correct size", () => {
		const list = new SortedList<number>((a, b) => a - b);
		list.insert(5);
		list.insert(1);
		list.insert(3);
		list.insert(2);
		list.insert(4);

		expect(list.size).toBe(5);
	});

	it("Sort on insert and return correct page", () => {
		const list = new SortedList<number>((a, b) => a - b);
		list.insert(5);
		list.insert(1);
		list.insert(3);
		list.insert(2);
		list.insert(4);

		let page = list.getPage(2);
		expect(page.items).toEqual([1, 2]);
		expect(page.nextCursor?.value).toEqual(3);

		page = list.getPage(2, page.nextCursor);
		expect(page.items).toEqual([3, 4]);
		expect(page.nextCursor?.value).toEqual(5);

		page = list.getPage(2, page.nextCursor);
		expect(page.items).toEqual([5]);
		expect(page.nextCursor).toBeUndefined();

		page = list.getPage(2, page.nextCursor);
		expect(page.items).toEqual([]);
		expect(page.nextCursor).toBeUndefined();
	});

	it("Reverse sort on insert and return correct page", () => {
		const list = new SortedList<number>((a, b) => b - a);
		list.insert(5);
		list.insert(1);
		list.insert(3);
		list.insert(2);
		list.insert(4);

		let page = list.getPage(2);
		expect(page.items).toEqual([5, 4]);
		expect(page.nextCursor?.value).toEqual(3);

		page = list.getPage(2, page.nextCursor);
		expect(page.items).toEqual([3, 2]);
        expect(page.nextCursor?.value).toEqual(1);

		page = list.getPage(2, page.nextCursor);
		expect(page.items).toEqual([1]);
        expect(page.nextCursor).toBeUndefined();

		page = list.getPage(2, page.nextCursor);
		expect(page.items).toEqual([]);
		expect(page.nextCursor).toBeUndefined();
	});
});
