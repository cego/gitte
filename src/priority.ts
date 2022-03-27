import { Project } from "./types/config";

export function getPriorityRange(projects: Project[]): { min: number, max: number } {

    const priorities = projects.reduce((carry, project) => {
        carry.push(project.priority ?? 0);
        Object.values(project.actions).forEach(action => {
            carry.push(action.priority ?? 0)
        });
        return carry;
    }, [] as number[]);
    return {min: Math.min(...priorities), max: Math.max(...priorities)};
}
