import Ajv from 'ajv';
const ajv = new Ajv();

const schema = {
    type: 'object',
    required: ['startup', 'projects'],
    properties: {
        "startup": {
            type: "object",
            additionalProperties: {
                anyOf: [
                    {
                        type: "object",
                        required: [
                            "cmd"
                        ],
                        properties: {
                            cmd: {
                                type: "array",
                                contains: { type: "string" },
                                // minContains: 1 todo
                            },
                            hint: {
                                type: "string"
                            },
                        },
                    },
                    {
                        type: "object",
                        required: [
                            "shell", "script"
                        ],
                        properties: {
                            shell: {
                                type: "string"
                            },
                            script: {
                                type: "string"
                            },
                            hint: {
                                type: "string"
                            },
                        },
                    }
                ]

            }
        },
        "projects": {
            type: "object",
            additionalProperties: {
                type: "object",
                required: [
                    "remote",
                    "default_branch",
                ],
                properties: {
                    "remote": {
                        type: "string"
                    },
                    "default_branch": {
                        type: "string"
                    },
                    "priority": {
                        type: "integer",
                        minimum: 0,
                        maximum: 1000
                    },
                    "actions": {
                        type: "object",
                        additionalProperties: {
                            type: "object",
                            properties: {
                                priority: {
                                    type: "integer",
                                    minimum: 0,
                                    maximum: 1000
                                },
                                groups: {
                                    type: "object",
                                    additionalProperties: {
                                        type: "array",
                                        contains: { type: "string" },
                                        // minContains: 1 todo
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

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
    startup: {[key:string]: (CmdAction | ShellAction)};
    projects:{[key:string]: Project}
}

const validate = ajv.compile<Config>(schema);

export function validateYaml(obj: any) {
    const valid = validate(obj);
    if (!valid) {
        console.error(validate.errors);
    }
    return valid;
}