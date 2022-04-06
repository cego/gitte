import chalk from "chalk";
import execa from "execa";

export function printHeader(header: string) {
	console.log();
	console.log(chalk`{bgCyan  BEGIN } {bold ${header}}`);
	console.log();
}

export function spawn(file: string, args?: string[], options?: execa.Options): execa.ExecaChildProcess {
	return execa(file, args, options);
}
