import { createActionGraphs, topologicalSort, topologicalSortActionGraph } from "../src/graph";
import { cnfStub } from "./utils/stubs";

describe("Graph", () => {
    describe("topologicalSortActionGraph", () => {
        test("It finds edges", () => {
            const mockSorter = jest.fn().mockReturnValue([]);
            topologicalSortActionGraph(cnfStub, "start", mockSorter);
            expect(mockSorter).toHaveBeenCalledWith(new Map<string, string[]>([["projecta", []]]), "start");
        });

        test("It finds edges with needs", () => {
            const mockSorter = jest.fn().mockReturnValue([]);
            topologicalSortActionGraph(cnfStub, "up", mockSorter);
            expect(mockSorter).toHaveBeenCalledWith(new Map<string, string[]>([["projectd", ["projecte"]],["projecte", []]]), "up");
        });

        test("It throws if action contains both needs and priority", () => {
            let cnf = { ...cnfStub };
            cnf.projects["projectd"].actions["up"].priority = 0
            expect(() => {
                topologicalSortActionGraph(cnf, "up", () => []);
            }).toThrow();
        });
    });

    describe("topologicalSort", () => {
        test("It throws error if cycle is detected", () => {
            console.log = jest.fn();
            console.table = jest.fn();
            const edges = new Map<string, string[]>();
            edges.set("a", ["b"]);
            edges.set("b", ["a"]);
            expect(() => topologicalSort(edges, "a")).toThrowError(/Cycle/);
        });

        test("It sorts correctly", () => {
            const edges = new Map<string, string[]>();
            edges.set("a", ["b"]);
            edges.set("b", ["c"]);
            edges.set("c", []);
            expect(topologicalSort(edges, "a")).toEqual(["c", "b", "a"]);
        });
    });
});