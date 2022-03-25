import util from "util";
import { spawn } from 'child_process';

export async function asyncSpawn(cmdArgs: string[], cwd = process.cwd()): Promise<{ stdout: string; stderr: string; output: string; status: number }> {
    return new Promise((resolve, reject) => {
        const cp = spawn(cmdArgs[0], cmdArgs.slice(1), {env: process.env, cwd});

        let output = "";
        let stdout = "";
        let stderr = "";

        cp.stderr.on("data", (buff) => {
            stderr += buff.toString();
            output += buff.toString();
        });
        cp.stdout.on("data", (buff) => {
            stdout += buff.toString();
            output += buff.toString();
        });
        cp.on("exit", (status) => {
            if ((status ?? 0) === 0) {
                return resolve({stdout, stderr, output, status: status ?? 0});
            }
            return reject(new ExitError(`${output !== "" ? output : "$? [" + status + "]"}`));
        });
        cp.on("error", (e) => {
            return reject(new ExitError(`'${JSON.stringify(cmdArgs)}' had errors\n${e}`));
        });

    });
}