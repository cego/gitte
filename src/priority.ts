import { Project } from "./validate_yaml";

export function getPriorityRange(projects: Project[]): { min: number, max: number } {

    const priorities = projects.reduce((priorities, project) => {
        priorities.push(project.priority ?? 0);
        Object.values(project.actions).forEach(action => {
            priorities.push(action.priority ?? 0)
        });
        return priorities;
    }, [] as number[]);
    return { min: Math.min(...priorities), max: Math.max(...priorities) };
} 