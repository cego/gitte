import { loadConfig } from "../../src/config_loader";
import chalk from "chalk";
import { errorHandler } from "../../src/error_handler";
import yargs from "yargs/yargs";
import { y } from "../yargs";

// noinspection JSUnusedGlobalSymbols
export const command = "completion";
// noinspection JSUnusedGlobalSymbols
export const describe = "stuff";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
    y.showCompletionScript();
}
