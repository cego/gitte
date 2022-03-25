import util from "util";
import { exec } from 'child_process';

export const asyncExec = util.promisify(exec);