const http = require('http');

let requestId = 1;

// --- Core API Communication (JSON-RPC) ---
async function sendQuery(dbName, query) {
  return new Promise((resolve, reject) => {
    const params = {
        db_name: dbName,
        query: query,
    };

    const postData = JSON.stringify({
        jsonrpc: '2.0',
        method: 'wwfs.ExecuteQuery',
        params: [params],
        id: requestId++,
    });

    const options = {
      hostname: 'localhost',
      port: 8080,
      path: '/rpc',
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(postData),
      },
    };

    const req = http.request(options, (res) => {
      let data = '';
      res.on('data', (chunk) => { data += chunk; });
      res.on('end', () => {
        try {
          const response = JSON.parse(data);
          resolve(response);
        } catch (e) {
            console.error(`Failed to parse server response: ${data}`);
            reject(`Failed to parse server response: ${data}`);
        }
      });
    });

    req.on('error', (e) => {
        console.error(`API request error: ${e.message}`);
        reject(`API request error: ${e.message}`);
    });
    req.write(postData);
    req.end();
  });
}

async function createDatabase(dbName) {
    console.log(`Creating database: ${dbName}...`);
    const createRes = await sendQuery('', `CREATE DATABASE ${dbName}`);
    if (createRes.error) {
        console.error(`Failed to create database: ${createRes.error.message || createRes.error}`);
        throw new Error("Database creation failed, cannot proceed.");
    }
    try {
        const createResult = JSON.parse(createRes.result.result);
        const privateKey = createResult.private_key;
        console.log(JSON.stringify({
            db_name: dbName,
            private_key: privateKey
        }, null, 2));
    } catch (e) {
        console.error(`Failed to parse CREATE DATABASE response: ${e.message}`);
        console.error(`Raw response:`, createRes);
        throw new Error("Could not parse private key, cannot proceed.");
    }
}

function main() {
    const arg = process.argv.find(a => a.startsWith('--db-name='));
    if (!arg) {
        console.error('Usage: node createdb.js --db-name=<your-db-name>');
        process.exit(1);
    }
    const dbName = arg.split('=')[1];
    createDatabase(dbName).catch(err => {
        console.error(err.message);
        process.exit(1);
    });
}

main();
