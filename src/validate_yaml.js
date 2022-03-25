const Ajv = require('ajv');
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
                                items: {
                                    type: "string"
                                }
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
                                        items: {
                                            type: "string"
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
}

const validate = ajv.compile(schema);

function validateYaml(obj) {
    const valid = validate(obj);
    if (!valid) {
        console.error(validate.errors);
    }
    return valid;
}

module.exports = { validateYaml }