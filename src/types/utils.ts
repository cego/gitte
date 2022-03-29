import * as pcp from "promisify-child-process";

export type ToChildProcessOutput = [(Error & pcp.Output) | null, pcp.Output | undefined];
