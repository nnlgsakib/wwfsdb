const http = require('http');
const crypto = require('crypto');

const dbName = 'testdb' + Date.now();

const testState = {
    passed: 0,
    failed: 0,
    requestId: 1,
    privateKey: null,
};

// --- Signing function ---
function sign(privateKeyHex, message) {
    try {
        const ecdh = crypto.createECDH('prime256v1');
        ecdh.setPrivateKey(privateKeyHex, 'hex');
        const publicKey = ecdh.getPublicKey('hex', 'uncompressed');

        const privateKeyJwk = {
            kty: 'EC',
            crv: 'P-256',
            d: Buffer.from(privateKeyHex, 'hex').toString('base64url'),
            x: Buffer.from(publicKey.substring(2, 66), 'hex').toString('base64url'),
            y: Buffer.from(publicKey.substring(66), 'hex').toString('base64url'),
        };

        const privateKeyObject = crypto.createPrivateKey({ key: privateKeyJwk, format: 'jwk' });

        const signer = crypto.createSign('sha256');
        signer.update(message);
        signer.end();

        const signature = signer.sign({ key: privateKeyObject, dsaEncoding: 'ieee-p1363' });

        return signature.toString('hex');
    } catch (e) {
        console.error(`
❌ Signing error: ${e.message}`);
        return null;
    }
}


// --- Core API Communication (JSON-RPC) ---
async function sendQuery(dbName, query, sessionID = null, privateKey = null) {
  return new Promise((resolve, reject) => {
    const params = {
        db_name: dbName,
        query: query,
    };
    if (sessionID) {
        params.session_id = sessionID;
    }

    // New: Add signature for write queries if private key is provided
    if (privateKey && !query.trim().toUpperCase().startsWith('SELECT') && !query.trim().toUpperCase().startsWith('BEGIN')) {
        const message = `${dbName}:${query}`;
        const signature = sign(privateKey, message);
        if (signature) {
            params.signature = signature;
        } else {
            // Signing failed, reject the promise
            return reject('Failed to sign the query.');
        }
    }

    const postData = JSON.stringify({
        jsonrpc: '2.0',
        method: 'wwfs.ExecuteQuery',
        params: [params],
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
        const data = JSON.parse(res.result.result);
        // If the result is an object with a 'rows' property, return the rows.
        if (data && typeof data === 'object' && Array.isArray(data.rows)) {
            return data.rows;
        }
        return data;
    } catch (e) {
        // This can happen for non-SELECT queries that return a simple string.
        return res.result.result;
    }
}

async function assertQueryResult(query, expected, message, { sessionID = null, privateKey = testState.privateKey } = {}) {
    const res = await sendQuery(dbName, query, sessionID, privateKey);
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

async function assertOrderedQueryResult(query, expected, message, { sessionID = null, privateKey = testState.privateKey } = {}) {
    const res = await sendQuery(dbName, query, sessionID, privateKey);
    if (res.error) {
        console.log(`  ❌ ${message}`);
        testState.failed++;
        console.log(`     Query failed unexpectedly: ${query}`);
        console.log(`     Error: ${res.error.message || res.error}`);
        return;
    }

    const data = safeParseResult(res);

    function orderedDeepEqual(a, b) {
        if (a === b) return true;
        if (typeof a !== 'object' || a === null || typeof b !== 'object' || b === null) return false;
        if (Array.isArray(a)) {
            if (!Array.isArray(b) || a.length !== b.length) return false;
            for (let i = 0; i < a.length; i++) {
                if (!orderedDeepEqual(a[i], b[i])) return false;
            }
            return true;
        }
        const keysA = Object.keys(a).sort();
        const keysB = Object.keys(b).sort();
        if (keysA.join(',') !== keysB.join(',')) return false;
        for (const key of keysA) {
            if (!orderedDeepEqual(a[key], b[key])) return false;
        }
        return true;
    }

    const success = orderedDeepEqual(data, expected);

    console.log(`  ${success ? '✅' : '❌'} ${message}`);

    if (!success) {
        testState.failed++;
        console.log('     Expected:', JSON.stringify(expected, null, 2));
        console.log('     Got:     ', JSON.stringify(data, null, 2));
    } else {
        testState.passed++;
    }
}

async function assertOrderedQueryResult(query, expected, message, { sessionID = null, privateKey = testState.privateKey } = {}) {
    const res = await sendQuery(dbName, query, sessionID, privateKey);
    if (res.error) {
        console.log(`  ❌ ${message}`);
        testState.failed++;
        console.log(`     Query failed unexpectedly: ${query}`);
        console.log(`     Error: ${res.error.message || res.error}`);
        return;
    }

    const data = safeParseResult(res);

    function orderedDeepEqual(a, b) {
        if (a === b) return true;
        if (typeof a !== 'object' || a === null || typeof b !== 'object' || b === null) return false;
        if (Array.isArray(a)) {
            if (!Array.isArray(b) || a.length !== b.length) return false;
            for (let i = 0; i < a.length; i++) {
                if (!orderedDeepEqual(a[i], b[i])) return false;
            }
            return true;
        }
        const keysA = Object.keys(a).sort();
        const keysB = Object.keys(b).sort();
        if (keysA.join(',') !== keysB.join(',')) return false;
        for (const key of keysA) {
            if (!orderedDeepEqual(a[key], b[key])) return false;
        }
        return true;
    }

    const success = orderedDeepEqual(data, expected);

    console.log(`  ${success ? '✅' : '❌'} ${message}`);

    if (!success) {
        testState.failed++;
        console.log('     Expected:', JSON.stringify(expected, null, 2));
        console.log('     Got:     ', JSON.stringify(data, null, 2));
    } else {
        testState.passed++;
    }
}

async function assertCommandSuccess(query, message, { sessionID = null, privateKey = testState.privateKey } = {}) {
    const res = await sendQuery(dbName, query, sessionID, privateKey);
    const data = safeParseResult(res);
    const upperQuery = query.trim().toUpperCase();
    let success = !res.error;

    if (success && (upperQuery.startsWith('UPDATE') || upperQuery.startsWith('DELETE'))) {
        success = typeof data === 'string' && /successful\. \d+ rows affected\.$/.test(data);
    }

    console.log(`  ${success ? '✅' : '❌'} ${message}`);
    if (!success) {
        testState.failed++;
        console.log(`     Query failed or returned unexpected result: ${query}`);
        if (res.error) {
            console.log(`     Error: ${res.error.message || res.error}`);
        } else {
            console.log(`     Got:     `, data);
            if (upperQuery.startsWith('UPDATE') || upperQuery.startsWith('DELETE')) {
                console.log(`     Expected a string like "UPDATE/DELETE successful. N rows affected."`);
            }
        }
    } else {
        testState.passed++;
    }
}

async function assertCommandFailure(query, message, { sessionID = null, privateKey = null } = {}) {
    const res = await sendQuery(dbName, query, sessionID, privateKey);
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
    // Manually handle database creation to capture the private key
    console.log(`  Creating database: ${dbName}...`);
    const createRes = await sendQuery('', `CREATE DATABASE ${dbName}`);
    if (createRes.error) {
        console.log(`  ❌ Failed to create database: ${createRes.error.message || createRes.error}`);
        testState.failed++;
        throw new Error("Database creation failed, cannot proceed.");
    }
    try {
        const createResult = JSON.parse(createRes.result.result);
        testState.privateKey = createResult.private_key;
        console.log(testState.privateKey );
        console.log(`  ✅ Creates a new database (IPNS: ${createResult.program_id})`);
        console.log(`     Private Key stored for subsequent tests.`);
        testState.passed++;
    } catch (e) {
        console.log(`  ❌ Failed to parse CREATE DATABASE response: ${e.message}`);
        testState.failed++;
        throw new Error("Could not parse private key, cannot proceed.");
    }


    const schema = `
    CREATE TABLE suppliers (supplier_id VARCHAR(255), name VARCHAR(255), contact_email VARCHAR(255), rating INT);
    CREATE TABLE products (product_id VARCHAR(255), name VARCHAR(255), description VARCHAR(255), unit_cost FLOAT, supplier_id VARCHAR(255));
    CREATE TABLE data_types_test (id INT, label VARCHAR(50), price FLOAT, is_active BOOLEAN);
  `;

    const createTableStatements = schema.split(';').filter(s => s.trim().length > 0);

    console.log('\n  Creating tables...');
    for (const statement of createTableStatements) {
        const query = statement.trim() + ';';
        // Use the stored private key for this write operation
        await assertCommandSuccess(query, `CREATE TABLE for ${query.match(/CREATE TABLE (\w+)/)[1]}`, { privateKey: testState.privateKey });
    }
}

async function runAuthTests() {
    console.log('\n--- 🧪 Running Authentication Tests ---\n');

    console.log('  --- Write Operations (Auth) ---');
    await assertCommandFailure(`INSERT INTO data_types_test VALUES (998, 'Auth Test 1', 1.0, true);`, 'Fails to insert a row without a private key', { privateKey: null });
    await assertCommandFailure(`INSERT INTO data_types_test VALUES (999, 'Auth Test 2', 1.0, true);`, 'Fails to insert a row with a wrong private key', { privateKey: 'a'.repeat(64) });
    await assertCommandSuccess(`INSERT INTO data_types_test VALUES (1000, 'Auth Test 3', 1.0, true);`, 'Succeeds to insert a row with the correct private key', { privateKey: testState.privateKey });

    console.log('\n  --- Read Operations (No Auth) ---');
    await assertQueryResult(`SELECT id FROM data_types_test WHERE id = 1000`, 
        [{ "id": 1000 }], 
        'Selects data without a private key', { privateKey: null });
}

async function runDataTypeTests() {
    console.log('\n--- 🧪 Running Data Type Tests ---\n');

    console.log('  --- INSERT Operations (Data Types) ---');
    await assertCommandSuccess(`INSERT INTO data_types_test VALUES (1, 'Item A', 99.99, true);`, 'Inserts correct types');
    await assertCommandSuccess(`INSERT INTO data_types_test VALUES (2, 'Item B', 120.50, false);`, 'Inserts more correct types');
    await assertCommandSuccess(`INSERT INTO data_types_test VALUES (3, 'Item C', -50, TRUE);`, 'Inserts with negative and uppercase boolean');

    console.log('\n  --- Type Validation on INSERT ---');
    await assertCommandFailure(`INSERT INTO data_types_test VALUES ('abc', 'Item D', 1.0, true);`, 'Fails to insert string into INT column');
    await assertCommandFailure(`INSERT INTO data_types_test VALUES (4, 'Item E', 'xyz', true);`, 'Fails to insert string into FLOAT column');
    await assertCommandFailure(`INSERT INTO data_types_test VALUES (5, 'Item F', 1.0, 'not-a-bool');`, 'Fails to insert invalid value into BOOLEAN column');

    console.log('\n  --- SELECT Operations (Verify Native Types) ---');
    await assertQueryResult(`SELECT * FROM data_types_test WHERE id = 1`, 
        [{ "data_types_test.id": 1, "data_types_test.label": "Item A", "data_types_test.price": 99.99, "data_types_test.is_active": true }], 
        'Selects row and verifies native types (INT, FLOAT, BOOLEAN)', { privateKey: null });

    console.log('\n  --- UPDATE Operations (Data Types) ---');
    await assertCommandSuccess(`UPDATE data_types_test SET price = 10.50 WHERE id = 1;`, 'Updates FLOAT with a valid value');
    await assertQueryResult(`SELECT price FROM data_types_test WHERE id = 1`, [{ "price": 10.50 }], 'Selects to confirm FLOAT update', { privateKey: null });
    
    await assertCommandSuccess(`UPDATE data_types_test SET is_active = false WHERE id = 1;`, 'Updates BOOLEAN with a valid value');
    await assertQueryResult(`SELECT is_active FROM data_types_test WHERE id = 1`, [{ "is_active": false }], 'Selects to confirm BOOLEAN update', { privateKey: null });

    console.log('\n  --- UPDATE Operations (Native Literals) ---');
    await assertCommandSuccess(`UPDATE data_types_test SET price = 20.75 WHERE id = 2;`, 'Updates FLOAT with a native float literal');
    await assertQueryResult(`SELECT price FROM data_types_test WHERE id = 2`, [{ "price": 20.75 }], 'Selects to confirm native FLOAT update', { privateKey: null });

    await assertCommandSuccess(`UPDATE data_types_test SET is_active = true WHERE id = 2;`, 'Updates BOOLEAN with a native boolean literal');
    await assertQueryResult(`SELECT is_active FROM data_types_test WHERE id = 2`, [{ "is_active": true }], 'Selects to confirm native BOOLEAN update', { privateKey: null });

    await assertCommandSuccess(`UPDATE data_types_test SET id = 10 WHERE id = 1;`, 'Updates INT with a native integer literal');
    await assertQueryResult(`SELECT id FROM data_types_test WHERE id = 10`, [{ "id": 10 }], 'Selects to confirm native INT update', { privateKey: null });

    console.log('\n  --- Type Validation on UPDATE ---');
    await assertCommandFailure(`UPDATE data_types_test SET id = 'not-an-int' WHERE label = 'Item B';`, 'Fails to update INT column with invalid string');
    await assertCommandFailure(`UPDATE data_types_test SET price = 'expensive' WHERE label = 'Item B';`, 'Fails to update FLOAT column with invalid string');
    await assertCommandFailure(`UPDATE data_types_test SET is_active = 'maybe' WHERE label = 'Item B';`, 'Fails to update BOOLEAN column with invalid string');

    console.log('\n  --- WHERE Clause Operations (Data Types) ---');
    await assertQueryResult(`SELECT id FROM data_types_test WHERE price > 100`, [], 'Selects using WHERE on a FLOAT column (after update)', { privateKey: null });
    await assertQueryResult(`SELECT id FROM data_types_test WHERE price < 0`, [{ "id": 3 }], 'Selects using WHERE with negative float', { privateKey: null });
    await assertQueryResult(`SELECT id FROM data_types_test WHERE is_active = true`, [{ "id": 1000 }, { "id": 2 }, { "id": 3 }], 'Selects using WHERE on a BOOLEAN column (after update)', { privateKey: null });
    await assertQueryResult(`SELECT id FROM data_types_test WHERE is_active = false`, [{ "id": 10 }], 'Selects using WHERE on a BOOLEAN column (after update)', { privateKey: null });
}

async function runRegressionTests() {
    console.log('\n--- 🧪 Running Regression Tests ---\n');

    console.log('  --- INSERT Operations ---');
    await assertCommandSuccess(`INSERT INTO suppliers VALUES ('sup1', 'Supplier A', 'contact@suppliera.com', 4);`, 'Inserts a new supplier');
    await assertCommandSuccess(`INSERT INTO products VALUES ('prod1', 'Product One', 'High-quality gadget', 19.99, 'sup1');`, 'Inserts a new product');

    console.log('\n  --- SELECT Operations ---');
    await assertQueryResult(`SELECT * FROM suppliers WHERE supplier_id = 'sup1'`, 
        [{ "suppliers.contact_email": "contact@suppliera.com", "suppliers.name": "Supplier A", "suppliers.rating": 4, "suppliers.supplier_id": "sup1" }], 
        'Selects supplier by ID, verifying native INT type', { privateKey: null });
    await assertQueryResult(`SELECT name, unit_cost FROM products WHERE product_id = 'prod1'`, 
        [{ "name": "Product One", "unit_cost": 19.99 }], 
        'Selects product by ID, verifying native FLOAT type', { privateKey: null });

    console.log('\n  --- UPDATE Operations ---');
    await assertCommandSuccess(`UPDATE products SET unit_cost = 25.50 WHERE product_id = 'prod1'`, 'Updates product cost');
    await assertQueryResult(`SELECT unit_cost FROM products WHERE product_id = 'prod1'`, [{ "unit_cost": 25.50 }], 'Selects updated product cost', { privateKey: null });

    console.log('\n  --- Advanced Queries ---');
    await assertQueryResult(`SELECT * FROM products WHERE unit_cost > 20`, 
        [{ "products.description": "High-quality gadget", "products.name": "Product One", "products.product_id": "prod1", "products.supplier_id": "sup1", "products.unit_cost": 25.50 }], 
        'Finds products with unit_cost > 20', { privateKey: null });
    
    await assertCommandSuccess(`INSERT INTO suppliers VALUES ('sup2', 'Supplier B', 'contact@supplierb.com', 5);`, 'Inserts a second supplier for IN test');
    await assertQueryResult(`SELECT name FROM suppliers WHERE rating > 4`, 
        [{ "name": "Supplier B"}], 
        'Finds suppliers with rating > 4', { privateKey: null });

    console.log('\n  --- DELETE Operations ---');
    await assertCommandSuccess(`DELETE FROM suppliers WHERE supplier_id = 'sup2'`, 'Deletes a supplier');
    await assertQueryResult(`SELECT * FROM suppliers WHERE supplier_id = 'sup2'`, [], 'Selects to confirm deletion', { privateKey: null });
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
        'Selects using an index on the WHERE clause column', { privateKey: null });

    console.log('\n  --- Index Maintenance ---');
    await assertCommandSuccess(`INSERT INTO suppliers VALUES ('sup3', 'Supplier C', 'contact@supplierc.com', 3);`, 'Inserts a new supplier (should update index)');
    await assertQueryResult(`SELECT supplier_id FROM suppliers WHERE name = 'Supplier C'`, 
        [{ "supplier_id": "sup3" }], 
        'Selects new supplier using the index', { privateKey: null });

    await assertCommandSuccess(`UPDATE suppliers SET name = 'Supplier C Updated' WHERE supplier_id = 'sup3';`, 'Updates a supplier name (should update index)');
    await assertQueryResult(`SELECT supplier_id FROM suppliers WHERE name = 'Supplier C Updated'`, 
        [{ "supplier_id": "sup3" }], 
        'Selects updated supplier using the index', { privateKey: null });
    await assertQueryResult(`SELECT * FROM suppliers WHERE name = 'Supplier C'`, 
        [], 
        'Selects old name to confirm index update', { privateKey: null });

    await assertCommandSuccess(`DELETE FROM suppliers WHERE name = 'Supplier A';`, 'Deletes a supplier (should update index)');
    await assertQueryResult(`SELECT * FROM suppliers WHERE name = 'Supplier A'`, 
        [], 
        'Selects deleted supplier to confirm index update', { privateKey: null });
}

async function runTransactionTests() {
    console.log('\n--- 🧪 Running Transaction Tests ---\n');

    let sessionID = null;

    console.log('  --- BEGIN Transaction ---');
    let res = await sendQuery(dbName, 'BEGIN', null, testState.privateKey);
    if (res.error) {
        console.log(`  ❌ Failed to BEGIN transaction: ${res.error.message || res.error}`);
        testState.failed++;
        return;
    }
    sessionID = res.result.session_id;
    console.log(`  ✅ BEGIN transaction (Session ID: ${sessionID})`);
    testState.passed++;

    console.log('\n  --- Operations within Transaction ---');
    await assertCommandSuccess(`INSERT INTO data_types_test VALUES (100, 'Transacted Item', 100.00, true);`, 'Inserts a new row within transaction', { sessionID });
    await assertCommandSuccess(`UPDATE data_types_test SET price = 150.00 WHERE id = 100;`, 'Updates a row within transaction', { sessionID });

    console.log('\n  --- Verify Isolation (Outside Transaction) ---');
    await assertQueryResult(`SELECT * FROM data_types_test WHERE id = 100`, [], 'Changes are NOT visible outside transaction', { privateKey: null });

    console.log('\n  --- COMMIT Transaction ---');
    await assertCommandSuccess('COMMIT;', 'Commits the transaction', { sessionID });
    sessionID = null; // Clear session ID after commit

    console.log('\n  --- Verify Changes After COMMIT ---');
    await assertQueryResult(`SELECT * FROM data_types_test WHERE id = 100`, 
        [{ "data_types_test.id": 100, "data_types_test.label": "Transacted Item", "data_types_test.price": 150.00, "data_types_test.is_active": true }], 
        'Changes ARE visible after commit', { privateKey: null });

    console.log('\n  --- BEGIN another Transaction for ROLLBACK ---');
    res = await sendQuery(dbName, 'BEGIN', null, testState.privateKey);
    if (res.error) {
        console.log(`  ❌ Failed to BEGIN transaction for rollback: ${res.error.message || res.error}`);
        testState.failed++;
        return;
    }
    sessionID = res.result.session_id;
    console.log(`  ✅ BEGIN transaction for rollback (Session ID: ${sessionID})`);
    testState.passed++;

    console.log('\n  --- Operations within Transaction (for Rollback) ---');
    await assertCommandSuccess(`INSERT INTO data_types_test VALUES (101, 'Rollback Item', 200.00, false);`, 'Inserts a new row for rollback', { sessionID });
    await assertCommandSuccess(`UPDATE data_types_test SET price = 250.00 WHERE id = 101;`, 'Updates a row for rollback', { sessionID });

    console.log('\n  --- Verify Isolation (Outside Transaction) before Rollback ---');
    await assertQueryResult(`SELECT * FROM data_types_test WHERE id = 101`, [], 'Changes are NOT visible outside transaction before rollback', { privateKey: null });

    console.log('\n  --- ROLLBACK Transaction ---');
    await assertCommandSuccess('ROLLBACK;', 'Rolls back the transaction', { sessionID });
    sessionID = null; // Clear session ID after rollback

    console.log('\n  --- Verify Changes After ROLLBACK ---');
    await assertQueryResult(`SELECT * FROM data_types_test WHERE id = 101`, [], 'Changes are NOT visible after rollback', { privateKey: null });

    console.log('\n  --- Test for invalid COMMIT/ROLLBACK without BEGIN ---');
    await assertCommandFailure('COMMIT;', 'Fails to COMMIT without an active transaction');
    await assertCommandFailure('ROLLBACK;', 'Fails to ROLLBACK without an active transaction');
}

async function runJoinTests() {
    console.log('\n--- 🧪 Running JOIN Tests ---\n');

    console.log('  --- Setting up data for JOIN tests ---');
    // Clean up previous data to ensure a clean slate
    await assertCommandSuccess(`DELETE FROM products WHERE 1=1;`, 'Cleans products table');
    await assertCommandSuccess(`DELETE FROM suppliers WHERE 1=1;`, 'Cleans suppliers table');

    // Insert new data
    await assertCommandSuccess(`INSERT INTO suppliers VALUES ('sup1', 'Join Supplier A', 'jsa@test.com', 5);`, 'Inserts supplier A');
    await assertCommandSuccess(`INSERT INTO suppliers VALUES ('sup2', 'Join Supplier B', 'jsb@test.com', 4);`, 'Inserts supplier B');
    await assertCommandSuccess(`INSERT INTO suppliers VALUES ('sup3', 'Join Supplier C', 'jsc@test.com', 3);`, 'Inserts supplier C (no products)');

    await assertCommandSuccess(`INSERT INTO products VALUES ('prod1', 'Join Product 1', 'From A', 10.0, 'sup1');`, 'Inserts product 1 for supplier A');
    await assertCommandSuccess(`INSERT INTO products VALUES ('prod2', 'Join Product 2', 'From A', 20.0, 'sup1');`, 'Inserts product 2 for supplier A');
    await assertCommandSuccess(`INSERT INTO products VALUES ('prod3', 'Join Product 3', 'From B', 30.0, 'sup2');`, 'Inserts product 3 for supplier B');

    console.log('\n  --- INNER JOIN Tests ---');
    await assertQueryResult(
        `SELECT suppliers.name, products.name FROM suppliers JOIN products ON suppliers.supplier_id = products.supplier_id;`,
        [
            { "suppliers.name": "Join Supplier A", "products.name": "Join Product 1" },
            { "suppliers.name": "Join Supplier A", "products.name": "Join Product 2" },
            { "suppliers.name": "Join Supplier B", "products.name": "Join Product 3" }
        ],
        'INNER JOIN selects qualified columns'
    );

    await assertQueryResult(
        `SELECT * FROM suppliers JOIN products ON suppliers.supplier_id = products.supplier_id WHERE products.unit_cost > 15;`,
        [
            { 
                "suppliers.supplier_id": "sup1", "suppliers.name": "Join Supplier A", "suppliers.contact_email": "jsa@test.com", "suppliers.rating": 5,
                "products.product_id": "prod2", "products.name": "Join Product 2", "products.description": "From A", "products.unit_cost": 20.0, "products.supplier_id": "sup1"
            },
            { 
                "suppliers.supplier_id": "sup2", "suppliers.name": "Join Supplier B", "suppliers.contact_email": "jsb@test.com", "suppliers.rating": 4,
                "products.product_id": "prod3", "products.name": "Join Product 3", "products.description": "From B", "products.unit_cost": 30.0, "products.supplier_id": "sup2"
            }
        ],
        'INNER JOIN with SELECT * and a WHERE clause'
    );

    console.log('\n  --- LEFT JOIN Tests ---');
    await assertQueryResult(
        `SELECT suppliers.name, products.name FROM suppliers LEFT JOIN products ON suppliers.supplier_id = products.supplier_id;`,
        [
            { "suppliers.name": "Join Supplier A", "products.name": "Join Product 1" },
            { "suppliers.name": "Join Supplier A", "products.name": "Join Product 2" },
            { "suppliers.name": "Join Supplier B", "products.name": "Join Product 3" },
            { "suppliers.name": "Join Supplier C", "products.name": null }
        ],
        'LEFT JOIN includes supplier with no products'
    );

    console.log('\n  --- Ambiguity Tests ---');
    // This test assumes the DB returns an error for ambiguous columns.
    await assertCommandFailure(
        `SELECT name FROM suppliers JOIN products ON suppliers.supplier_id = products.supplier_id;`,
        'Fails to select ambiguous column "name" without qualifier'
    );
}

async function runAlterTableTests() {
    console.log('\n--- 🧪 Running ALTER TABLE Tests ---\n');

    // Setup a specific table for ALTER tests
    await assertCommandSuccess(`CREATE TABLE alter_test (id INT, name VARCHAR(50));`, 'Creates a table for ALTER tests');
    await assertCommandSuccess(`INSERT INTO alter_test VALUES (1, 'initial_name');`, 'Inserts initial data into alter_test');

    console.log('\n  --- ADD COLUMN ---');
    await assertCommandSuccess(`ALTER TABLE alter_test ADD COLUMN status VARCHAR(20);`, 'Adds a new column "status"');
    
    // Verify by inserting data into the new column
    await assertCommandSuccess(`INSERT INTO alter_test VALUES (2, 'new_row', 'active');`, 'Inserts a row with data for the new column');
    
    // Verify by updating the new column on an old row
    await assertCommandSuccess(`UPDATE alter_test SET status = 'inactive' WHERE id = 1;`, 'Updates the new column on an existing row');
    
    // Verify the result
    await assertQueryResult(
        `SELECT id, status FROM alter_test`,
        [
            { "id": 1, "status": "inactive" },
            { "id": 2, "status": "active" }
        ],
        'Selects data from new and updated column'
    );

    console.log('\n  --- RENAME COLUMN ---');
    await assertCommandSuccess(`ALTER TABLE alter_test RENAME COLUMN name TO full_name;`, 'Renames column "name" to "full_name"');

    // The old 'name' column data is now inaccessible under the new schema for old rows.
    // Selecting the new column name for an old row should result in null.
    await assertQueryResult(
        `SELECT id, full_name FROM alter_test WHERE id = 1`,
        [{ "id": 1, "full_name": null }],
        'Selects renamed column for old row (should be null)'
    );

    // We can, however, update the new column name
    await assertCommandSuccess(`UPDATE alter_test SET full_name = 'name_updated' WHERE id = 1;`, 'Updates a value using the new column name');
    await assertQueryResult(
        `SELECT id, full_name FROM alter_test WHERE id = 1`,
        [{ "id": 1, "full_name": "name_updated" }],
        'Selects the updated value from the renamed column'
    );

    console.log('\n  --- DROP COLUMN ---');
    await assertCommandSuccess(`ALTER TABLE alter_test DROP COLUMN status;`, 'Drops the column "status"');

    // Verify the column is gone by trying to select it, which should fail.
    await assertCommandFailure(`SELECT id, status FROM alter_test WHERE id = 1;`, 'Fails to select a dropped column');
    
    // Verify we can still select other columns
    await assertQueryResult(
        `SELECT id, full_name FROM alter_test WHERE id = 1`,
        [{ "id": 1, "full_name": "name_updated" }],
        'Selects other columns after a column is dropped'
    );
}

async function runAdvancedQueryTests() {
    console.log('\n--- 🧪 Running Advanced Query (ORDER BY, GROUP BY, Aggregates) Tests ---\n');

    console.log('  --- Setting up data for advanced query tests ---');
    await assertCommandSuccess(`CREATE TABLE sales (id INT, category VARCHAR(50), region VARCHAR(50), amount FLOAT, quantity INT);`, 'Creates a table for advanced query tests');
    
    const salesData = [
        `(1, 'electronics', 'north', 120.50, 2)`,
        `(2, 'books', 'south', 15.00, 3)`,
        `(3, 'electronics', 'north', 75.00, 1)`,
        `(4, 'clothing', 'west', 45.99, 5)`,
        `(5, 'books', 'north', 25.00, 2)`,
        `(6, 'clothing', 'south', 80.25, 2)`,
        `(7, 'electronics', 'west', 250.00, 1)`,
        `(8, 'books', 'south', 10.00, 1)`
    ];

    for (let i = 0; i < salesData.length; i++) {
        await assertCommandSuccess(`INSERT INTO sales VALUES ${salesData[i]};`, `Inserts sales data row ${i + 1}`);
    }

    console.log('\n  --- ORDER BY Tests ---');
    await assertOrderedQueryResult(
        `SELECT id FROM sales ORDER BY amount DESC;`,
        [{"id": 7}, {"id": 1}, {"id": 6}, {"id": 3}, {"id": 4}, {"id": 5}, {"id": 2}, {"id": 8}],
        'Selects IDs ordered by amount descending'
    );
    await assertOrderedQueryResult(
        `SELECT id FROM sales ORDER BY category ASC, amount DESC;`,
        [{"id": 5}, {"id": 2}, {"id": 8}, {"id": 6}, {"id": 4}, {"id": 7}, {"id": 1}, {"id": 3}],
        'Selects IDs ordered by category ascending, then amount descending'
    );

    console.log('\n  --- LIMIT / OFFSET Tests ---');
    await assertOrderedQueryResult(
        `SELECT id FROM sales ORDER BY amount DESC LIMIT 3;`,
        [{"id": 7}, {"id": 1}, {"id": 6}],
        'Selects top 3 sales by amount using LIMIT'
    );
    await assertOrderedQueryResult(
        `SELECT id FROM sales ORDER BY amount DESC OFFSET 2;`,
        [{"id": 6}, {"id": 3}, {"id": 4}, {"id": 5}, {"id": 2}, {"id": 8}],
        'Selects sales by amount, skipping the top 2 using OFFSET'
    );
    await assertOrderedQueryResult(
        `SELECT id FROM sales ORDER BY amount DESC LIMIT 2 OFFSET 3;`,
        [{"id": 3}, {"id": 4}],
        'Selects 2 sales by amount, skipping the top 3 (pagination)'
    );

    console.log('\n  --- Aggregate Function Tests (No GROUP BY) ---');
    await assertQueryResult(`SELECT COUNT(*) FROM sales;`, [{"COUNT(*)": 8}], 'Selects COUNT(*) of all rows');
    await assertQueryResult(`SELECT SUM(quantity) FROM sales;`, [{"SUM(quantity)": 17}], 'Selects SUM() of quantity');
    await assertQueryResult(`SELECT AVG(amount) FROM sales;`, [{"AVG(amount)": 77.7175}], 'Selects AVG() of amount');
    await assertQueryResult(`SELECT MIN(amount) FROM sales;`, [{"MIN(amount)": 10.00}], 'Selects MIN() of amount');
    await assertQueryResult(`SELECT MAX(amount) FROM sales;`, [{"MAX(amount)": 250.00}], 'Selects MAX() of amount');

    console.log('\n  --- GROUP BY Tests ---');
    await assertOrderedQueryResult(
        `SELECT category, COUNT(*) FROM sales GROUP BY category ORDER BY category ASC;`,
        [
            { "category": "books", "COUNT(*)": 3 },
            { "category": "clothing", "COUNT(*)": 2 },
            { "category": "electronics", "COUNT(*)": 3 }
        ],
        'Selects COUNT(*) grouped by category'
    );
    await assertOrderedQueryResult(
        `SELECT region, SUM(amount) FROM sales GROUP BY region ORDER BY region ASC;`,
        [
            { "region": "north", "SUM(amount)": 220.5 },
            { "region": "south", "SUM(amount)": 105.25 },
            { "region": "west", "SUM(amount)": 295.99 }
        ],
        'Selects SUM(amount) grouped by region'
    );
    await assertOrderedQueryResult(
        `SELECT category, AVG(quantity) FROM sales GROUP BY category ORDER BY category ASC;`,
        [
            { "category": "books", "AVG(quantity)": 2 },
            { "category": "clothing", "AVG(quantity)": 3.5 },
            { "category": "electronics", "AVG(quantity)": 1.3333333333333333 }
        ],
        'Selects AVG(quantity) grouped by category'
    );
    await assertOrderedQueryResult(
        `SELECT region, MIN(amount), MAX(amount) FROM sales GROUP BY region ORDER BY region ASC;`,
        [
            { "region": "north", "MIN(amount)": 25.00, "MAX(amount)": 120.50 },
            { "region": "south", "MIN(amount)": 10.00, "MAX(amount)": 80.25 },
            { "region": "west", "MIN(amount)": 45.99, "MAX(amount)": 250.00 }
        ],
        'Selects MIN() and MAX() grouped by region'
    );
}

async function runWhitespaceAndCommentTests() {
    console.log('\n--- 🧪 Running Whitespace & Comment Tests ---\n');

    // Create a test table first
    await assertCommandSuccess(`CREATE TABLE comment_test (id INT, name VARCHAR(50), value FLOAT);`, 'Creates a table for comment and whitespace tests');
    
    console.log('  --- Testing Multiline SQL Statements ---');
    // Test multiline SQL with various indentation
    const multilineQuery = `
        INSERT INTO comment_test 
        VALUES 
            (1, 
             'Multiline Test', 
             123.45);`;
    await assertCommandSuccess(multilineQuery, 'Executes multiline SQL with indentation');

    // Test multiline SELECT with indentation
    await assertQueryResult(
        `SELECT 
             id, 
             name, 
             value 
         FROM comment_test 
         WHERE id = 1;`,
        [{"id": 1, "name": "Multiline Test", "value": 123.45}],
        'Executes multiline SELECT with indentation'
    );

    console.log('\n  --- Testing SQL with Comments ---');
    // Test SQL with single-line comments
    await assertCommandSuccess(`INSERT INTO comment_test -- This is a comment
                                VALUES (2, 'Comment Test', 99.99);`, 'Executes SQL with single-line comment');

    // Test SQL with inline comments
    await assertQueryResult(
        `SELECT id /* inline comment */, name FROM comment_test WHERE value > 100; -- This finds values > 100`,
        [{"id": 1, "name": "Multiline Test"}],
        'Executes SELECT with both inline and end-of-line comments'
    );

    // Test multiline comment
    await assertCommandSuccess(`INSERT INTO comment_test VALUES /* multiline
    comment
    spanning
    lines */ (3, 'Multiline Comment Test', 75.25);`, 'Executes SQL with multiline comment');
    
    await assertQueryResult(
        `SELECT name FROM comment_test WHERE id = 3;`,
        [{"name": "Multiline Comment Test"}],
        'Confirms data from multiline comment SQL'
    );

    console.log('\n  --- Testing Various Indentation Styles ---');
    // Test different indentation patterns
    const indentedQueries = [
        `INSERT INTO comment_test VALUES
            (4, 'Tab Indent', 10.0);`,
        `   INSERT INTO comment_test VALUES
              (5, 'Space Indent', 20.0);`,
        `INSERT INTO comment_test VALUES 
             (6, 'Continuation Line', 30.0);`
    ];

    for (let i = 0; i < indentedQueries.length; i++) {
        await assertCommandSuccess(indentedQueries[i], `Executes indented query ${i + 1}`);
    }

    // Verify all inserted records
    await assertQueryResult(
        `SELECT COUNT(*) FROM comment_test;`,
        [{"COUNT(*)": 6}],
        'Verifies all records were inserted despite formatting variations'
    );

    console.log('\n  --- Testing Complex Formatting ---');
    // Complex example mixing all features
    await assertCommandSuccess(`
        -- This is a complex test combining various formatting features
        INSERT INTO comment_test VALUES 
               (7, 
               'Complex Format' /* inline comment */,
               42.0); -- end of line comment`,
        'Executes complex formatted SQL with comments and newlines');
    
    await assertQueryResult(
        `SELECT name, value FROM comment_test WHERE id = 7; -- with comment`,
        [{"name": "Complex Format", "value": 42.0}],
        'Verifies complex formatted SQL result'
    );
}

async function runAdvancedLiteralTests() {
    console.log('\n--- 🧪 Running Advanced Literal Tests (Phase 1 Features) ---\n');

    // Create a test table for advanced literals
    await assertCommandSuccess(`CREATE TABLE literal_test (
        id INT, 
        name VARCHAR(50),
        hex_value INT,
        bin_value INT,
        sci_value FLOAT,
        unicode_name VARCHAR(50)
    );`, 'Creates a table for advanced literal tests');
    
    console.log('  --- Testing Scientific Notation Numbers ---');
    // Test scientific notation literals
    await assertCommandSuccess(`INSERT INTO literal_test VALUES 
        (1, 'Sci Notation Test', 0xFF, 0b1010, 1.23e-4, 'αβγ_delta');`, 
        'Inserts values with scientific notation, hex, binary, and Unicode');

    await assertQueryResult(
        `SELECT sci_value FROM literal_test WHERE id = 1;`,
        [{"sci_value": 0.000123}],
        'Selects and verifies scientific notation value (1.23e-4 = 0.000123)'
    );

    console.log('\n  --- Testing Hexadecimal Literals ---');
    // Test hex literals
    await assertCommandSuccess(`INSERT INTO literal_test VALUES 
        (2, 'Hex Test', 0x1A2F, 0b1111, 2.5E+5, 'ñame_ü');`, 
        'Inserts values with hex and binary literals');

    await assertQueryResult(
        `SELECT hex_value FROM literal_test WHERE id = 2;`,
        [{"hex_value": 6703}], // 0x1A2F = 6703 in decimal
        'Selects and verifies hexadecimal value (0x1A2F = 6703)'
    );

    console.log('\n  --- Testing Binary Literals ---');
    // Test binary literals
    await assertCommandSuccess(`INSERT INTO literal_test VALUES 
        (3, 'Binary Test', 0XFF, 0B11111111, 3.14e10, 'unicode_θ');`, 
        'Inserts values with uppercase hex and binary literals');

    await assertQueryResult(
        `SELECT bin_value FROM literal_test WHERE id = 3;`,
        [{"bin_value": 255}], // 0b11111111 = 255 in decimal
        'Selects and verifies binary value (0b11111111 = 255)'
    );

    console.log('\n  --- Testing Unicode Identifiers ---');
    // Test Unicode in string literals
    await assertCommandSuccess(`INSERT INTO literal_test VALUES 
        (4, 'Unicode Test: αβγ_δ', 0x0, 0b0, 1.0, 'café_Москва');`, 
        'Inserts values with Unicode characters in string literals');

    await assertQueryResult(
        `SELECT name FROM literal_test WHERE id = 4;`,
        [{"name": "Unicode Test: αβγ_δ"}],
        'Selects and verifies Unicode characters in string literal'
    );

    console.log('\n  --- Testing Mixed Literal Formats ---');
    // Complex example with all literal types
    await assertCommandSuccess(`
        -- Complex test with mixed literal formats
        INSERT INTO literal_test VALUES 
               (5, 
               'Mixed Format Test' /* inline comment */,
               0xDEADBEEF,    -- hex literal
               0b10101010,    -- binary literal
               6.022e23,      -- scientific notation
               'unicode_ñ_ü_θ'); -- Unicode string
    `, 'Executes complex formatted SQL with mixed literal types');
    
    await assertQueryResult(
        `SELECT hex_value, bin_value, sci_value FROM literal_test WHERE id = 5;`,
        [{"hex_value": 3735928559, "bin_value": 170, "sci_value": 6.022e23}],
        'Verifies complex formatted SQL result with mixed literal types'
    );

    console.log('\n  --- Testing DATE/TIME Keywords ---');
    // Test that DATE, TIME, TIMESTAMP keywords work as identifiers
    await assertCommandSuccess(`CREATE TABLE "DATE" (id INT, "TIME" VARCHAR(50), "TIMESTAMP" FLOAT);`, 
        'Creates table with DATE/TIME/TIMESTAMP as quoted identifiers');

    await assertCommandSuccess(`INSERT INTO "DATE" VALUES (1, '2023-01-01 12:00:00', 1672567200.0);`, 
        'Inserts values into table with reserved keywords as column names');

    await assertQueryResult(
        `SELECT "TIME" FROM "DATE" WHERE id = 1;`,
        [{"TIME": "2023-01-01 12:00:00"}],
        'Selects from table with reserved keywords as column names'
    );

    // Clean up
    await assertCommandSuccess(`DROP TABLE "DATE";`, 'Drops table with quoted identifier');
}

async function main() {
  try {
    console.log('Assuming Go server is running in a separate terminal.');
    await new Promise(resolve => setTimeout(resolve, 1000));

    await setupDatabase();
    await runAuthTests();
    await runDataTypeTests();
    await runRegressionTests();
    await runIndexingTests();
    await runTransactionTests();
    await runAlterTableTests();
    await runJoinTests();
    await runAdvancedQueryTests();
    await runWhitespaceAndCommentTests(); // Test Phase 0 enhancements
    await runAdvancedLiteralTests(); // Test Phase 1 enhancements

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