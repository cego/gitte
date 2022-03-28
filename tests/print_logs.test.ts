import chalk from "chalk";
import { printLogs } from "../src/utils";

beforeEach(() => {
    console.log = jest.fn();
});

describe("Print logs", () => {
    test("It logs all successful", async () => {
        const projectNames = ["test1", "test2",];
        const logs: any[][] = [
            ["log1", "log2"],
            ["log3"],
        ];

        printLogs(projectNames, logs);

        expect(console.log).toHaveBeenCalledTimes(5);
        expect(console.log).toHaveBeenCalledWith(chalk`┌─ {green {bold test1}}`);
        expect(console.log).toHaveBeenCalledWith(`├─── log1`);
        expect(console.log).toHaveBeenCalledWith(`└─── log2`);
        expect(console.log).toHaveBeenCalledWith(chalk`┌─ {green {bold test2}}`);
        expect(console.log).toHaveBeenCalledWith(`└─── log3`);
    })

    test("It logs all failed", async () => {
        const projectNames = ["test1", "test2",];
        const logs: any[] = [
            new Error("test error 1"),
            new Error("test error 2"),
        ];

        expect(() => printLogs(projectNames, logs)).toThrowError("At least one git operation failed");

        expect(console.log).toHaveBeenCalledTimes(4);
        expect(console.log).toHaveBeenCalledWith(chalk`┌─ {red {bold test1}}`);
        expect(console.log).toHaveBeenCalledWith(
            expect.stringContaining("Error: test error 1")
        );
        expect(console.log).toHaveBeenCalledWith(chalk`┌─ {red {bold test2}}`);
        expect(console.log).toHaveBeenCalledWith(
            expect.stringContaining("Error: test error 2")
        );
    });

    test("It logs all failed and successful", async () => {
        const projectNames = ["test1", "test2",];
        const logs: any[] = [
            new Error("test error 1"),
            ["log3"],
        ];

        expect(() => printLogs(projectNames, logs)).toThrowError("At least one git operation failed");

        expect(console.log).toHaveBeenCalledTimes(4);
        expect(console.log).toHaveBeenCalledWith(chalk`┌─ {red {bold test1}}`);
        expect(console.log).toHaveBeenCalledWith(
            expect.stringContaining("Error: test error 1")
        );
        expect(console.log).toHaveBeenCalledWith(chalk`┌─ {green {bold test2}}`);
        expect(console.log).toHaveBeenCalledWith(`└─── log3`);
    });
});