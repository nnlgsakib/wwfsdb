const http = require('http');

const dbName = 'testdb' + Date.now();

const testState = {
    passed: 0,
    failed: 0,
    requestId: 1,
};

// --- Core API Communication (JSON-RPC) ---
async function sendQuery(dbName, query) {
  return new Promise((resolve, reject) => {
    const postData = JSON.stringify({
        jsonrpc: '2.0',
        method: 'wwfs.ExecuteQuery',
        params: [{
            db_name: dbName,
            query: query,
        }],
        id: testState.requestId++,
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
          // console.log('RAW RESPONSE:', JSON.stringify(response, null, 2)); // Uncomment for debugging
          resolve(response);
        } catch (e) {
            console.error(`
❌ Failed to parse server response: ${data}`);
            reject(`Failed to parse server response: ${data}`);
        }
      });
    });

    req.on('error', (e) => {
        console.error(`
❌ API request error: ${e.message}`);
        reject(`API request error: ${e.message}`);
    });
    req.write(postData);
    req.end();
  });
}

function safeParseResult(res) {
    if (!res || !res.result || !res.result.result) {
        return [];
    }
    try {
        return JSON.parse(res.result.result);
    } catch (e) {
        // This can happen for non-SELECT queries that return a simple string.
        return res.result.result;
    }
}

async function assertQueryResult(query, expected, message) {
    const res = await sendQuery(dbName, query);
    if (res.error) {
        console.log(`  ❌ ${message}`);
        testState.failed++;
        console.log(`     Query failed unexpectedly: ${query}`);
        console.log(`     Error: ${res.error.message || res.error}`);
        return;
    }

    const data = safeParseResult(res);

    const Mismatch = {
        Value: 'value',
        Type: 'type',
    };

    function findMismatch(a, b) {
        if (typeof a !== typeof b) return { mismatch: Mismatch.Type, path: '', a, b };
        if (typeof a !== 'object' || a === null) {
            return a === b ? null : { mismatch: Mismatch.Value, path: '', a, b };
        }
        if (Array.isArray(a)) {
            if (!Array.isArray(b) || a.length !== b.length) return { mismatch: Mismatch.Value, path: '', a, b };
            const bCopy = [...b];
            for (const itemA of a) {
                const foundIndex = bCopy.findIndex(itemB => findMismatch(itemA, itemB) === null);
                if (foundIndex === -1) return { mismatch: Mismatch.Value, path: '', a, b };
                bCopy.splice(foundIndex, 1);
            }
            return null;
        }
        const keysA = Object.keys(a).sort();
        const keysB = Object.keys(b).sort();
        if (keysA.join(',') !== keysB.join(',')) return { mismatch: Mismatch.Value, path: '', a, b };
        for (const key of keysA) {
            const mismatch = findMismatch(a[key], b[key]);
            if (mismatch) {
                mismatch.path = `.${key}${mismatch.path}`;
                return mismatch;
            }
        }
        return null;
    }

    const mismatch = findMismatch(data, expected);
    const success = mismatch === null;

    console.log(`  ${success ? '✅' : '❌'} ${message}`);

    if (!success) {
        testState.failed++;
        console.log('     Expected:', JSON.stringify(expected));
        console.log('     Got:     ', JSON.stringify(data));
        if (mismatch) {
            console.log(`     Mismatch Details: ${mismatch.mismatch} at path '${mismatch.path}' (Expected: ${JSON.stringify(mismatch.b)}, Got: ${JSON.stringify(mismatch.a)})`);
        }
    } else {
        testState.passed++;
    }
}

async function assertCommandSuccess(query, message) {
    const res = await sendQuery(dbName, query);
    const success = !res.error;
    console.log(`  ${success ? '✅' : '❌'} ${message}`);
    if (!success) {
        testState.failed++;
        console.log(`     Query failed: ${query}`);
        console.log(`     Error: ${res.error.message || res.error}`);
    } else {
        testState.passed++;
    }
}

async function assertCommandFailure(query, message) {
    const res = await sendQuery(dbName, query);
    const success = !!res.error;
    console.log(`  ${success ? '✅' : '❌'} ${message}`);
    if (!success) {
        testState.failed++;
        console.log(`     Query was expected to fail but succeeded: ${query}`);
    } else {
        testState.passed++;
    }
}

// --- Test Groups ---
async function setupDatabase() {
  console.log(`
--- 🚀 Setting up database: ${dbName} ---
`);
  await assertCommandSuccess(`CREATE DATABASE ${dbName}`, 'Creates a new database');

  const schema = `
    CREATE TABLE suppliers (supplier_id VARCHAR(255), name VARCHAR(255), contact_email VARCHAR(255), rating INT);
    CREATE TABLE products (product_id VARCHAR(255), name VARCHAR(255), description VARCHAR(255), unit_cost FLOAT, supplier_id VARCHAR(255));
    CREATE TABLE data_types_test (id INT, label VARCHAR(50), price FLOAT, is_active BOOLEAN);
  `;
  
  const createTableStatements = schema.split(';').filter(s => s.trim().length > 0);

  console.log('\n  Creating tables...');
  for (const statement of createTableStatements) {
      const query = statement.trim() + ';';
      await assertCommandSuccess(query, `CREATE TABLE for ${query.match(/CREATE TABLE (\w+)/)[1]}`);
  }
}

async function runDataTypeTests() {
    console.log('\n--- 🧪 Running Data Type Tests ---\n');

    console.log('  --- INSERT Operations (Data Types) ---');
    await assertCommandSuccess(`INSERT INTO data_types_test VALUES ('1', 'Item A', '99.99', 'true');`, 'Inserts correct types as strings');
    await assertCommandSuccess(`INSERT INTO data_types_test VALUES ('2', 'Item B', '120.50', 'false');`, 'Inserts more correct types');
    await assertCommandSuccess(`INSERT INTO data_types_test VALUES ('3', 'Item C', '-50', 'TRUE');`, 'Inserts with negative and uppercase boolean');

    console.log('\n  --- Type Validation on INSERT ---');
    await assertCommandFailure(`INSERT INTO data_types_test VALUES ('abc', 'Item D', '1.0', 'true');`, 'Fails to insert string into INT column');
    await assertCommandFailure(`INSERT INTO data_types_test VALUES ('4', 'Item E', 'xyz', 'true');`, 'Fails to insert string into FLOAT column');
    await assertCommandFailure(`INSERT INTO data_types_test VALUES ('5', 'Item F', '1.0', 'not-a-bool');`, 'Fails to insert invalid value into BOOLEAN column');

    console.log('\n  --- SELECT Operations (Verify Native Types) ---');
    await assertQueryResult(`SELECT * FROM data_types_test WHERE id = 1`, 
        [{ "id": 1, "label": "Item A", "price": 99.99, "is_active": true }], 
        'Selects row and verifies native types (INT, FLOAT, BOOLEAN)');

    console.log('\n  --- UPDATE Operations (Data Types) ---');
    await assertCommandSuccess(`UPDATE data_types_test SET price = '10.50' WHERE id = 1;`, 'Updates FLOAT with a valid string');
    await assertQueryResult(`SELECT price FROM data_types_test WHERE id = 1`, [{ "price": 10.50 }], 'Selects to confirm FLOAT update');
    
    await assertCommandSuccess(`UPDATE data_types_test SET is_active = 'false' WHERE id = 1;`, 'Updates BOOLEAN with a valid string');
    await assertQueryResult(`SELECT is_active FROM data_types_test WHERE id = 1`, [{ "is_active": false }], 'Selects to confirm BOOLEAN update');

    console.log('\n  --- Type Validation on UPDATE ---');
    await assertCommandFailure(`UPDATE data_types_test SET id = 'not-an-int' WHERE label = 'Item B';`, 'Fails to update INT column with invalid string');
    await assertCommandFailure(`UPDATE data_types_test SET price = 'expensive' WHERE label = 'Item B';`, 'Fails to update FLOAT column with invalid string');
    await assertCommandFailure(`UPDATE data_types_test SET is_active = 'maybe' WHERE label = 'Item B';`, 'Fails to update BOOLEAN column with invalid string');

    console.log('\n  --- WHERE Clause Operations (Data Types) ---');
    await assertQueryResult(`SELECT id FROM data_types_test WHERE price > 100`, [{ "id": 2 }], 'Selects using WHERE on a FLOAT column');
    await assertQueryResult(`SELECT id FROM data_types_test WHERE price < 0`, [{ "id": 3 }], 'Selects using WHERE with negative float');
    await assertQueryResult(`SELECT id FROM data_types_test WHERE is_active = true`, [{ "id": 3 }], 'Selects using WHERE on a BOOLEAN column (after update)');
    await assertQueryResult(`SELECT id FROM data_types_test WHERE is_active = false`, [{ "id": 1 }, { "id": 2 }], 'Selects using WHERE on a BOOLEAN column');
}

async function runRegressionTests() {
    console.log('\n--- 🧪 Running Regression Tests ---\n');

    console.log('  --- INSERT Operations ---');
    await assertCommandSuccess(`INSERT INTO suppliers VALUES ('sup1', 'Supplier A', 'contact@suppliera.com', '4');`, 'Inserts a new supplier');
    await assertCommandSuccess(`INSERT INTO products VALUES ('prod1', 'Product One', 'High-quality gadget', '19.99', 'sup1');`, 'Inserts a new product');

    console.log('\n  --- SELECT Operations ---');
    await assertQueryResult(`SELECT * FROM suppliers WHERE supplier_id = 'sup1'`, 
        [{ "contact_email": "contact@suppliera.com", "name": "Supplier A", "rating": 4, "supplier_id": "sup1" }], 
        'Selects supplier by ID, verifying native INT type');
    await assertQueryResult(`SELECT name, unit_cost FROM products WHERE product_id = 'prod1'`, 
        [{ "name": "Product One", "unit_cost": 19.99 }], 
        'Selects product by ID, verifying native FLOAT type');

    console.log('\n  --- UPDATE Operations ---');
    await assertCommandSuccess(`UPDATE products SET unit_cost = '25.50' WHERE product_id = 'prod1'`, 'Updates product cost');
    await assertQueryResult(`SELECT unit_cost FROM products WHERE product_id = 'prod1'`, [{ "unit_cost": 25.50 }], 'Selects updated product cost');

    console.log('\n  --- Advanced Queries ---');
    await assertQueryResult(`SELECT * FROM products WHERE unit_cost > 20`, 
        [{ "description": "High-quality gadget", "name": "Product One", "product_id": "prod1", "supplier_id": "sup1", "unit_cost": 25.50 }], 
        'Finds products with unit_cost > 20');
    
    await assertCommandSuccess(`INSERT INTO suppliers VALUES ('sup2', 'Supplier B', 'contact@supplierb.com', '5');`, 'Inserts a second supplier for IN test');
    await assertQueryResult(`SELECT name FROM suppliers WHERE rating > 4`, 
        [{ "name": "Supplier B"}], 
        'Finds suppliers with rating > 4');

    console.log('\n  --- DELETE Operations ---');
    await assertCommandSuccess(`DELETE FROM suppliers WHERE supplier_id = 'sup2'`, 'Deletes a supplier');
    await assertQueryResult(`SELECT * FROM suppliers WHERE supplier_id = 'sup2'`, [], 'Selects to confirm deletion');
}

async function runIndexingTests() {
    console.log('\n--- 🧪 Running Indexing Tests ---\n');

    console.log('  --- CREATE INDEX ---');
    await assertCommandSuccess(`CREATE INDEX ON suppliers (name);`, 'Creates an index on the suppliers.name column');
    await assertCommandFailure(`CREATE INDEX ON suppliers (name);`, 'Fails to create an index that already exists');
    await assertCommandFailure(`CREATE INDEX ON suppliers (non_existent_column);`, 'Fails to create an index on a non-existent column');

    console.log('\n  --- Query using Index ---');
    await assertQueryResult(`SELECT supplier_id FROM suppliers WHERE name = 'Supplier A'`, 
        [{ "supplier_id": "sup1" }], 
        'Selects using an index on the WHERE clause column');

    console.log('\n  --- Index Maintenance ---');
    await assertCommandSuccess(`INSERT INTO suppliers VALUES ('sup3', 'Supplier C', 'contact@supplierc.com', '3');`, 'Inserts a new supplier (should update index)');
    await assertQueryResult(`SELECT supplier_id FROM suppliers WHERE name = 'Supplier C'`, 
        [{ "supplier_id": "sup3" }], 
        'Selects new supplier using the index');

    await assertCommandSuccess(`UPDATE suppliers SET name = 'Supplier C Updated' WHERE supplier_id = 'sup3';`, 'Updates a supplier name (should update index)');
    await assertQueryResult(`SELECT supplier_id FROM suppliers WHERE name = 'Supplier C Updated'`, 
        [{ "supplier_id": "sup3" }], 
        'Selects updated supplier using the index');
    await assertQueryResult(`SELECT * FROM suppliers WHERE name = 'Supplier C'`, 
        [], 
        'Selects old name to confirm index update');

    await assertCommandSuccess(`DELETE FROM suppliers WHERE name = 'Supplier A';`, 'Deletes a supplier (should update index)');
    await assertQueryResult(`SELECT * FROM suppliers WHERE name = 'Supplier A'`, 
        [], 
        'Selects deleted supplier to confirm index update');
}

async function main() {
  try {
    console.log('Assuming Go server is running in a separate terminal.');
    await new Promise(resolve => setTimeout(resolve, 1000));

    await setupDatabase();
    await runDataTypeTests();
    await runRegressionTests();
    await runIndexingTests();

  } catch (e) {
    console.error('\n--- A critical error occurred ---');
    console.error(e);
    testState.failed++;
  } finally {
    console.log('\n--- 🏁 Test Run Finished ---');
    console.log(`    Total tests: ${testState.passed + testState.failed}`);
    console.log(`    ✅ Passed:    ${testState.passed}`);
    console.log(`    ❌ Failed:    ${testState.failed}`);
    console.log('---------------------------');
    process.exit(testState.failed > 0 ? 1 : 0);
  }
}

main();
