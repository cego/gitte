export type Action = {
	hint?: string;
};
export type CmdAction = Action & { cmd: [string, ...string[]] };
export type ShellAction = Action & { shell: string; script: string };

export type ProjectAction = {
	searchFor: SearchFor[];
	priority: number | null;
	needs: string[];
	groups: { [key: string]: [string, ...string[]] };
};

export type Project = {
	common?: boolean;
	remote: string;
	default_branch: string;
	actions: { [key: string]: ProjectAction };
	defaultDisabled?: boolean;
};

export type ActionOverride = {
	maxParallelization?: number;
};

export type Config = {
	cwd: string;
	needs: boolean;
	startup: { [key: string]: CmdAction | ShellAction };
	switch?: { upAction: string; downAction: string };
	actionOverride?: { [key: string]: ActionOverride };
	projects: { [key: string]: Project };
	searchFor: SearchFor[];
};

export type SearchFor = {
	regex: string;
	hint: string;
};
