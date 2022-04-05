import { ActionOutputPrinter } from "../src/actions_utils";
import { loadConfig } from "../src/config_loader";
import * as utils from "../src/utils";
import {ExecaReturnValue } from "execa";

describe("ActionOutputPrinter", () => {
    test("It runs actions and prints output", async () => {
        // @ts-ignore
        utils.spawn = jest.fn();
        const spawnSpy = jest.spyOn(utils, "spawn").mockResolvedValue({ stdout: "Mocked Stdout" } as unknown as ExecaReturnValue<string>);

        const cnf = await loadConfig(process.cwd());
        const actionOutputPrinter = new ActionOutputPrinter(cnf, "start", "cego.dk");
        const output = await actionOutputPrinter.run();

        expect(output).toBe("");
    });
});