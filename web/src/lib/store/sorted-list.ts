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
	private count = 0;

	constructor(compare: (a: T, b: T) => number) {
		this.compare = compare;
	}

	insert(value: T): void {
		this.root = this.insertNode(this.root, value);
		this.count++;
	}

	get size(): number {
		return this.count;
	}

	get(index: number): T | undefined {
		if (index < 0 || index >= this.count) {
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

	getPage(
		size: number,
		cursor?: TreeNode<T>,
	): { items: T[]; nextCursor?: TreeNode<T> } {
		const result: T[] = [];
		let count = 0;
		let foundCursor = false;
		let nextCursor: TreeNode<T> | undefined;

		function traverse(node: TreeNode<T> | null): void {
			if (node === null) {
				return;
			}

			traverse(node.left);

			if (cursor) {
				if (!foundCursor) {
					if (node === cursor) {
						foundCursor = true;
					}
				} else {
					if (count < size) {
						result.push(node.value);
						count++;
					} else if (!nextCursor) {
						nextCursor = node;
					}
				}
			} else {
				if (count < size) {
					result.push(node.value);
					count++;
				} else if (!nextCursor) {
					nextCursor = node;
				}
			}

			traverse(node.right);
		}

		traverse(this.root);

		return {
			items: result,
			nextCursor: count >= size ? nextCursor : undefined,
		};
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
