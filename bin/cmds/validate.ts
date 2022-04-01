import chalk from "chalk";
import {loadConfig} from "../../src/config_loader";
import {errorHandler} from "../../src/error_handler";

// noinspection JSUnusedGlobalSymbols
export const command = "validate";
// noinspection JSUnusedGlobalSymbols
export const describe = "Validate the configuration";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
    try {
        await loadConfig(argv.cwd);
        console.log(chalk`{green .git-local-devops.yml is valid}`);
    } catch (e) {
        errorHandler(e)
    }

}
