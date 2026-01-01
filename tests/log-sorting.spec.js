"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var __generator = (this && this.__generator) || function (thisArg, body) {
    var _ = { label: 0, sent: function() { if (t[0] & 1) throw t[1]; return t[1]; }, trys: [], ops: [] }, f, y, t, g = Object.create((typeof Iterator === "function" ? Iterator : Object).prototype);
    return g.next = verb(0), g["throw"] = verb(1), g["return"] = verb(2), typeof Symbol === "function" && (g[Symbol.iterator] = function() { return this; }), g;
    function verb(n) { return function (v) { return step([n, v]); }; }
    function step(op) {
        if (f) throw new TypeError("Generator is already executing.");
        while (g && (g = 0, op[0] && (_ = 0)), _) try {
            if (f = 1, y && (t = op[0] & 2 ? y["return"] : op[0] ? y["throw"] || ((t = y["return"]) && t.call(y), 0) : y.next) && !(t = t.call(y, op[1])).done) return t;
            if (y = 0, t) op = [op[0] & 2, t.value];
            switch (op[0]) {
                case 0: case 1: t = op; break;
                case 4: _.label++; return { value: op[1], done: false };
                case 5: _.label++; y = op[1]; op = [0]; continue;
                case 7: op = _.ops.pop(); _.trys.pop(); continue;
                default:
                    if (!(t = _.trys, t = t.length > 0 && t[t.length - 1]) && (op[0] === 6 || op[0] === 2)) { _ = 0; continue; }
                    if (op[0] === 3 && (!t || (op[1] > t[0] && op[1] < t[3]))) { _.label = op[1]; break; }
                    if (op[0] === 6 && _.label < t[1]) { _.label = t[1]; t = op; break; }
                    if (t && _.label < t[2]) { _.label = t[2]; _.ops.push(op); break; }
                    if (t[2]) _.ops.pop();
                    _.trys.pop(); continue;
            }
            op = body.call(thisArg, _);
        } catch (e) { op = [6, e]; y = 0; } finally { f = t = 0; }
        if (op[0] & 5) throw op[1]; return { value: op[0] ? op[1] : void 0, done: true };
    }
};
Object.defineProperty(exports, "__esModule", { value: true });
var test_1 = require("@playwright/test");
var node_fs_1 = require("node:fs");
var node_path_1 = require("node:path");
var scriptPath = node_path_1.default.join(process.cwd(), "internal/server/ui/assets/js/log-line-swapping.js");
var clientScript = node_fs_1.default.readFileSync(scriptPath, "utf8");
test_1.test.describe("Log Sorting Logic", function () {
    test_1.test.beforeEach(function (_a) { return __awaiter(void 0, [_a], void 0, function (_b) {
        var page = _b.page;
        return __generator(this, function (_c) {
            switch (_c.label) {
                case 0: 
                // Setup a clean DOM with the container and your script
                return [4 /*yield*/, page.setContent("\n      <div id=\"logs-container\"></div>\n      <script>".concat(clientScript, "</script>\n    "))];
                case 1:
                    // Setup a clean DOM with the container and your script
                    _c.sent();
                    return [2 /*return*/];
            }
        });
    }); });
    // Helper to fire the specific HTMX event we are mocking
    var simulateHtmxSwap = function (page, timestamp) { return __awaiter(void 0, void 0, void 0, function () {
        return __generator(this, function (_a) {
            switch (_a.label) {
                case 0: return [4 /*yield*/, page.evaluate(function (ts) {
                        // Create the fragment and element exactly like HTMX would
                        var frag = document.createDocumentFragment();
                        var el = document.createElement("pre");
                        el.setAttribute("data-timestamp", ts);
                        el.innerText = "Log at ".concat(ts);
                        frag.appendChild(el);
                        // Dispatch the event
                        var evt = new CustomEvent("htmx:oobBeforeSwap", {
                            bubbles: true,
                            cancelable: true,
                            detail: {
                                target: { id: "logs-container" },
                                fragment: frag,
                            },
                        });
                        document.body.dispatchEvent(evt);
                    }, timestamp)];
                case 1:
                    _a.sent();
                    return [2 /*return*/];
            }
        });
    }); };
    (0, test_1.test)("handles out-of-order log arrival correctly", function (_a) { return __awaiter(void 0, [_a], void 0, function (_b) {
        var timestamps;
        var page = _b.page;
        return __generator(this, function (_c) {
            switch (_c.label) {
                case 0: 
                // 1. Receive First Log (Time: 100)
                return [4 /*yield*/, simulateHtmxSwap(page, 100)];
                case 1:
                    // 1. Receive First Log (Time: 100)
                    _c.sent();
                    // 2. Receive NEWER Log (Time: 300) -> Should go to TOP
                    return [4 /*yield*/, simulateHtmxSwap(page, 300)];
                case 2:
                    // 2. Receive NEWER Log (Time: 300) -> Should go to TOP
                    _c.sent();
                    // 3. Receive OLDER Log (Time: 50) -> Should go to BOTTOM
                    return [4 /*yield*/, simulateHtmxSwap(page, 50)];
                case 3:
                    // 3. Receive OLDER Log (Time: 50) -> Should go to BOTTOM
                    _c.sent();
                    // 4. Receive MIDDLE Log (Time: 200) -> Should insert BETWEEN 300 and 100
                    return [4 /*yield*/, simulateHtmxSwap(page, 200)];
                case 4:
                    // 4. Receive MIDDLE Log (Time: 200) -> Should insert BETWEEN 300 and 100
                    _c.sent();
                    return [4 /*yield*/, page
                            .locator("#logs-container > pre")
                            .evaluateAll(function (list) {
                            return list.map(function (el) { return parseInt(el.getAttribute("data-timestamp")); });
                        })];
                case 5:
                    timestamps = _c.sent();
                    console.log("Final DOM Order:", timestamps);
                    (0, test_1.expect)(timestamps).toEqual([300, 200, 100, 50]);
                    return [2 /*return*/];
            }
        });
    }); });
});
