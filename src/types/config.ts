export type Action = {
	hint?: string;
};
export type CmdAction = Action & { cmd: [string, ...string[]] };
export type ShellAction = Action & { shell: string; script: string };

export type ProjectAction = {
	priority?: number;
	needs?: string[];
	groups: { [key: string]: [string, ...string[]] };
};

export type Project = {
	remote: string;
	default_branch: string;
	actions: { [key: string]: ProjectAction };
};

export type Config = {
	cwd: string;
	startup: { [key: string]: CmdAction | ShellAction };
	projects: { [key: string]: Project };
	searchFor: SearchFor[];
};

export type SearchFor = {
	regex: string;
	hint: string;
};
