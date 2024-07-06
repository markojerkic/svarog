import { createStore } from "solid-js/store";
import type { LogLine, LogPageCursor } from "~/lib/store/log-store";

class TreeNode<T> {
	value: T;
	left: TreeNode<T> | null = null;
	right: TreeNode<T> | null = null;

	constructor(value: T) {
		this.value = value;
	}
}
export function treeNodeToCursor(
	node: TreeNode<LogLine> | null,
): LogPageCursor | null {
	if (!node) {
		return null;
	}

	return {
		cursorSequenceNumber: node.value.timestamp,
		cursorTime: node.value.timestamp,
		direction: "forward",
	};
}

export class SortedList<T> {
	private root: TreeNode<T> | null = null;
	private compare: (a: T, b: T) => number;
	private countStore = createStore({ count: 0 });
	private head: TreeNode<T> | null = null;
	private tail: TreeNode<T> | null = null;

	constructor(compare: (a: T, b: T) => number) {
		this.compare = compare;
	}

	insert(value: T): void {
		this.root = this.insertNode(this.root, value);
		this.updateHeadTail(value);
		this.countStore[1]("count", (prev) => prev + 1);
	}

	insertMany(values: T[]): void {
		for (const value of values) {
			this.insert(value);
		}
	}

	get size() {
		return this.countStore[0].count;
	}

	getHead() {
		return this.head;
	}

	getTail() {
		return this.tail;
	}

	get(index: number): T | undefined {
		if (index < 0 || index >= this.size) {
			return undefined;
		}

		let i = index;
		let node = this.root;
		while (node) {
			const leftCount = this.countNodes(node.left);
			if (i === leftCount) {
				return node.value;
			}
			if (i < leftCount) {
				node = node.left;
			} else {
				node = node.right;
				i -= leftCount + 1;
			}
		}

		return undefined;
	}

	private updateHeadTail(value: T): void {
		if (!this.head || this.compare(value, this.head.value) < 0) {
			this.head = this.findNode(this.root, value);
		}
		if (!this.tail || this.compare(value, this.tail.value) > 0) {
			this.tail = this.findNode(this.root, value);
		}
	}

	private findNode(node: TreeNode<T> | null, value: T): TreeNode<T> | null {
		if (node === null) {
			return null;
		}
		if (this.compare(value, node.value) === 0) {
			return node;
		}
		if (this.compare(value, node.value) < 0) {
			return this.findNode(node.left, value);
		}
		return this.findNode(node.right, value);
	}

	private countNodes(node: TreeNode<T> | null): number {
		if (node === null) {
			return 0;
		}
		return 1 + this.countNodes(node.left) + this.countNodes(node.right);
	}

	private insertNode(node: TreeNode<T> | null, value: T): TreeNode<T> {
		if (node === null) {
			return new TreeNode(value);
		}

		if (this.compare(value, node.value) < 0) {
			node.left = this.insertNode(node.left, value);
		} else {
			node.right = this.insertNode(node.right, value);
		}

		return node;
	}
}
