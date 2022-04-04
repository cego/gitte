const ansi = require("ansi-escape-sequences");

const sleep = async (ms) => {
    return new Promise((resolve) => {
        setTimeout(resolve, ms);
    });
};

(async () => {
    console.log("test")
    // sleep 1 second
    await sleep(1000);
    process.stdout.write(ansi.cursor.up(1));
    await sleep(1000);
})()