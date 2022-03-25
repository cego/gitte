import * as childProcess from "child_process";

export class ChildProcessError extends Error {
    stderr: string = "";
}

export type ChildProcessResult = {
    stdout: string;
}

export class Utils {
    static spawn(cmd: string, cmdArgs: string[], options: childProcess.SpawnOptions): Promise<{ stdout: string; stderr: string; output: string; status: number }> {
        return new Promise((resolve, reject) => {
            const cp = childProcess.spawn(cmd, cmdArgs, options);

            let output = "";
            let stdout = "";
            let stderr = "";

            cp.stderr?.on("data", (buff) => {
                stderr += buff.toString();
                output += buff.toString();
            });
            cp.stdout?.on("data", (buff) => {
                stdout += buff.toString();
                output += buff.toString();
            });
            cp.on("exit", (status) => {
                if ((status ?? 0) === 0) {
                    return resolve({stdout, stderr, output, status: status ?? 0});
                }
                return reject(new Error(`${output !== "" ? output : "$? [" + status + "]"}`));
            });
            cp.on("error", (e) => {
                return reject(new Error(`'${JSON.stringify(cmdArgs)}' had errors\n${e}`));
            });

        });
    }
}