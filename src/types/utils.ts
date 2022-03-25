export class ChildProcessError extends Error {
    stderr: string = "";
}

export type ChildProcessResult = {
    stdout: string;
}
