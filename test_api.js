const http = require('http');

async function sendQuery(dbName, query) {
  return new Promise((resolve, reject) => {
    const postData = JSON.stringify({
      db_name: dbName,
      query: query,
    });

    const options = {
      hostname: 'localhost',
      port: 8080,
      path: '/query',
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(postData),
      },
    };

    const req = http.request(options, (res) => {
      let data = '';
      res.on('data', (chunk) => {
        data += chunk;
      });
      res.on('end', () => {
        resolve(JSON.parse(data));
      });
    });

    req.on('error', (e) => {
      reject(e);
    });

    req.write(postData);
    req.end();
  });
}

async function testApi() {
  const dbName = `http_test_db_${Math.floor(Math.random() * 1000)}`;
  try {
    console.log(`1. Creating database ${dbName}...`);
    const createDbRes = await sendQuery('', `CREATE DATABASE ${dbName}`);
    console.log(createDbRes);

    console.log('\n2. Creating table...');
    const createTableRes = await sendQuery(dbName, 'CREATE TABLE users (id INT, name STRING);');
    console.log(createTableRes);

    console.log('\n3. Inserting data...');
    const insertRes = await sendQuery(dbName, "INSERT INTO users VALUES ('1', 'John Doe');");
    console.log(insertRes);

    console.log('\n4. Querying data...');
    const selectRes = await sendQuery(dbName, 'SELECT * FROM users');
    console.log(selectRes);

  } catch (error) {
    console.error('Error during API test:', error);
  }
}

testApi();
