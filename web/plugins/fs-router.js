import { glob } from "glob";
import path from "node:path";
import fs from "node:fs";
import { createFilter } from "@rollup/pluginutils";

export default function viteSolidFsRouter(options = {}) {
	const { pagesDir = "src/routes", extensions = ["jsx", "tsx"] } = options;

	let routesCode = "";
	const filter = createFilter(`${pagesDir}/**/*.{${extensions.join(",")}}`, []);

	function generateRoutePath(relativePath) {
		return relativePath
			.replace(/\.[^/.]+$/, "") // Remove extension
			.replace(/index$/, "") // Remove index
			.replace(/\[([^\]]+)\]/g, ":$1") // Convert [param] to :param
			.split(path.sep)
			.join("/");
	}

	return {
		name: "vite-plugin-solid-fs-router",

		configResolved(config) {
			const absolutePagesPath = path.resolve(config.root, pagesDir);
			if (!fs.existsSync(absolutePagesPath)) {
				fs.mkdirSync(absolutePagesPath, { recursive: true });
			}
		},

		async transform(code, id) {
			if (!filter(id)) return null;

			const moduleCode = `
        ${code}
        export const __routeMetadata = { ...route };
      `;

			return {
				code: moduleCode,
				map: null,
			};
		},

		async buildStart() {
			const absolutePagesPath = path.resolve(process.cwd(), pagesDir);
			const pattern = `${absolutePagesPath}/**/*.{${extensions.join(",")}}`;

			const files = await glob(pattern);

			files.sort((a, b) => {
				const depthA = a.split(path.sep).length;
				const depthB = b.split(path.sep).length;
				return depthA - depthB;
			});

			const imports = files
				.map((file, index) => {
					const importPath = `/${path.relative(process.cwd(), file)}`;
					return `
          import Page${index}, * as RouteConfig${index} from '${importPath}';
          const __routeMeta${index} = ('route' in RouteConfig${index} && RouteConfig${index}?.route) ?? {};
        `;
				})
				.join("\n");

			console.log(imports);

			const routes = files.map((file) => {
				const relativePath = path.relative(absolutePagesPath, file);
				const routePath = generateRoutePath(relativePath);
				const finalRoutePath = `/${routePath}`;

				return {
					path: finalRoutePath,
					depth: finalRoutePath.split("/").filter(Boolean).length,
					importPath: `/${path.relative(process.cwd(), file)}`,
				};
			});

			routesCode = `
        import { lazy } from 'solid-js';
        ${imports}

        export const routes = [
          ${routes
						.map(
							(route, index) => `
          {
            path: '${route.path}',
            component: lazy(() => import('${route.importPath}')),
            ...__routeMeta${index},
            ${route.depth > 1 ? "children: []" : ""}
          }`,
						)
						.join(",")}
        ];
      `;
		},

		resolveId(id) {
			if (id === "virtual:solid-routes") {
				return "\0virtual:solid-routes";
			}
		},

		load(id) {
			if (id === "\0virtual:solid-routes") {
				return routesCode;
			}
		},

		config(_config) {
			return {
				resolve: {
					alias: {
						"/src": path.resolve(process.cwd(), "src"),
					},
				},
			};
		},
	};
}
