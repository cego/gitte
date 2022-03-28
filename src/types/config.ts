export type Action = {
    hint: string | undefined;
}
export type CmdAction = Action & { cmd: [string, ...string[]] }
export type ShellAction = Action & { shell: string, script: string }

export type ProjectAction = {
    priority: number | undefined;
    groups: { [key: string]: [string, ...string[]] };
}

export type Project = {
    remote: string;
    default_branch: string;
    priority: number | undefined;
    actions: { [key: string]: ProjectAction };
}

export type Config = {
    startup: { [key: string]: (CmdAction | ShellAction) };
    projects: { [key: string]: Project };
    searchFor: SearchFor[];
}

export type SearchFor = {
    regex: string;
    hint: string;
}
