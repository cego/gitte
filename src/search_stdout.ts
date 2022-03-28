import chalk from "chalk";
import { SearchFor } from "./types/config";

export function searchStdoutAndPrintHints(
    searchFor: SearchFor[],
    stdoutHistory: {[key: string]: string[]}
){
    searchFor.forEach(search => searchForRegex(search, stdoutHistory));
}

function searchForRegex(searchFor: SearchFor, stdoutHistory: { [key: string]: string[]; }): void {
    const regex = new RegExp(searchFor.regex);
    for(const [action,stdout] of Object.entries(stdoutHistory)){
        for(const line of stdout){
            if(regex.test(line)){
                console.log(chalk`{yellow Hint: ${searchFor.hint}} {gray (Source: ${action})}`);
                return;
            }
        }
    }
}
