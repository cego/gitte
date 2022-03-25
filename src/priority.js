function getPriorityRange(projects) {

    const priorities = projects.reduce((priorities, project) => {
        priorities.push(project["priority"] ?? 0);
        Object.values(project.actions).forEach(action => {
            priorities.push(action.priority ?? 0)
        });
        return priorities;
    }, []);
    return { min: Math.min(...priorities), max: Math.max(...priorities) };
}

module.exports = { getPriorityRange };