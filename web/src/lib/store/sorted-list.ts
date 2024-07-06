import { createStore } from "solid-js/store";

class TreeNode<T> {
	value: T;
	left: TreeNode<T> | null = null;
	right: TreeNode<T> | null = null;

	constructor(value: T) {
		this.value = value;
	}
}

export class SortedList<T> {
	private root: TreeNode<T> | null = null;
	private compare: (a: T, b: T) => number;
	private countStore = createStore({ count: 0 });

	constructor(compare: (a: T, b: T) => number) {
		this.compare = compare;
	}

	insert(value: T): void {
		this.root = this.insertNode(this.root, value);
		this.countStore[1]("count", (prev) => prev + 1);
	}

	get size() {
		return this.countStore[0].count;
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
