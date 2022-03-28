
import { Config } from "./types/config";
import * as pcp from "promisify-child-process";
import path from "path";
import assert from "assert";
import dotenv from "dotenv";
import { validateYaml } from "./validate_yaml";
import fs from "fs-extra";
import yaml from "js-yaml";

export async function loadConfig(cwd: string): Promise<Config> {
    const cnfPath = `${cwd}/.git-local-devops.yml`;
    const dotenvPath = `${cwd}/.git-local-devops-env`;

    let fileContent;

    if (await fs.pathExists(dotenvPath)) {
        const envCnf = dotenv.parse(await fs.readFile(dotenvPath)); // will return an object
        assert(envCnf['REMOTE_GIT_PROJECT'], `REMOTE_GIT_PROJECT isn't defined in ${dotenvPath}`);
        assert(envCnf['REMOTE_GIT_PROJECT_FILE'], `REMOTE_GIT_PROJECT_FILE isn't defined in ${dotenvPath}`);
        await fs.ensureDir("/tmp/git-local-devops");
        await pcp.spawn(
            "git", ["archive", `--remote=${envCnf['REMOTE_GIT_PROJECT']}`, "master", envCnf['REMOTE_GIT_PROJECT_FILE'], "|", "tar", "-xC", "/tmp/git-local-devops/"],
            { shell: "bash", cwd, env: process.env, encoding: "utf8" },
        );
        fileContent = await fs.readFile(`/tmp/git-local-devops/${envCnf['REMOTE_GIT_PROJECT_FILE']}`, "utf8");
    } else if (await fs.pathExists(cnfPath)) {
        fileContent = await fs.readFile(cnfPath, "utf8");
    }
    else if (cwd === "/") {
        throw new Error(`No .git-local-devops.yml or .git-local-devops-env found in current or parent directories.`);
    }
    else {
        return loadConfig(path.resolve(cwd, '..'));
    }

    const yml: any = yaml.load(fileContent);
    assert(validateYaml(yml), "Invalid .git-local-devops.yml file");

    return yml as Config;
}