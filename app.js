const http = require('http');

const dbName = 'testdb' + Date.now();

const testState = {
    passed: 0,
    failed: 0,
};

// --- Core API Communication ---
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
      res.on('data', (chunk) => { data += chunk; });
      res.on('end', () => {
        try {
          const response = JSON.parse(data);
          if (response.error) {
            // This is now the primary error reporting location
            console.error(`\n❌ Query failed for: ${query}`);
            console.error(`   Error: ${response.error}`);
          }
          resolve(response);
        } catch (e) {
            console.error(`\n❌ Failed to parse server response: ${data}`);
            reject(`Failed to parse server response: ${data}`);
        }
      });
    });

    req.on('error', (e) => {
        console.error(`\n❌ API request error: ${e.message}`);
        reject(`API request error: ${e.message}`);
    });
    req.write(postData);
    req.end();
  });
}

function safeParseResult(res) {
    if (!res || !res.result || res.result === 'null') {
        return [];
    }
    try {
        const result = JSON.parse(res.result);
        if (Array.isArray(result)) {
            result.forEach(item => {
                if (typeof item === 'object' && item !== null) {
                    const sortedItem = {};
                    Object.keys(item).sort().forEach(key => { sortedItem[key] = item[key]; });
                    item = sortedItem;
                }
            });
            return result.sort((a, b) => JSON.stringify(a).localeCompare(JSON.stringify(b)));
        }
        return result;
    } catch (e) {
        console.error("Failed to parse result:", res.result);
        return [];
    }
}

async function assertQueryResult(query, expected, message) {
    const res = await sendQuery(dbName, query);
    const data = safeParseResult(res);
    
    if (Array.isArray(expected)) {
        expected.sort((a, b) => JSON.stringify(a).localeCompare(JSON.stringify(b)));
    }

    const success = JSON.stringify(data) === JSON.stringify(expected);
    
    console.log(`  ${success ? '✅' : '❌'} ${message}`);

    if (!success) {
        testState.failed++;
        console.log('     Expected:', JSON.stringify(expected));
        console.log('     Got:     ', JSON.stringify(data));
        // We don't exit immediately to see all failures
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
    } else {
        testState.passed++;
    }
}

// --- Test Groups ---
async function setupDatabase() {
  console.log(`\n--- 🚀 Setting up database: ${dbName} ---\n`);
  await sendQuery('', `CREATE DATABASE ${dbName}`);

  const schema = `
    CREATE TABLE suppliers (supplier_id VARCHAR(255), name VARCHAR(255), contact_email VARCHAR(255), rating INT);
    CREATE TABLE products (product_id VARCHAR(255), name VARCHAR(255), description VARCHAR(255), unit_cost FLOAT, supplier_id VARCHAR(255));
    CREATE TABLE warehouses (warehouse_id VARCHAR(255), name VARCHAR(255), location VARCHAR(255), capacity INT);
    CREATE TABLE inventory (inventory_id VARCHAR(255), product_id VARCHAR(255), warehouse_id VARCHAR(255), quantity INT, last_updated VARCHAR(255));
    CREATE TABLE shipments (shipment_id VARCHAR(255), product_id VARCHAR(255), from_warehouse_id VARCHAR(255), to_warehouse_id VARCHAR(255), quantity INT, status VARCHAR(100));
  `;
  
  const createTableStatements = schema.split(';').filter(s => s.trim().length > 0);

  console.log('  Creating tables...');
  for (const statement of createTableStatements) {
      const query = statement.trim() + ';';
      await assertCommandSuccess(query, `CREATE TABLE for ${query.match(/CREATE TABLE (\w+)/)[1]}`);
  }
}

async function runTests() {
    console.log('\n--- 🧪 Running All Tests ---\n');

    console.log('  --- INSERT Operations ---');
    await assertCommandSuccess(`INSERT INTO suppliers VALUES ('sup1', 'Supplier A', 'contact@suppliera.com', '4');`, 'Inserts a new supplier');
    await assertCommandSuccess(`INSERT INTO products VALUES ('prod1', 'Product One', 'High-quality gadget', '19.99', 'sup1');`, 'Inserts a new product');
    await assertCommandSuccess(`INSERT INTO warehouses VALUES ('wh1', 'Main Warehouse', 'New York, NY', '10000');`, 'Inserts a new warehouse');
    await assertCommandSuccess(`INSERT INTO inventory VALUES ('inv1', 'prod1', 'wh1', '500', '${new Date().toISOString()}');`, 'Inserts initial inventory');

    console.log('\n  --- SELECT Operations ---');
    await assertQueryResult(`SELECT * FROM suppliers WHERE supplier_id = 'sup1'`, [{ "contact_email": "contact@suppliera.com", "name": "Supplier A", "rating": "4", "supplier_id": "sup1" }], 'Selects supplier by ID');
    await assertQueryResult(`SELECT name, unit_cost FROM products WHERE product_id = 'prod1'`, [{ "name": "Product One", "unit_cost": "19.99" }], 'Selects product by ID');

    console.log('\n  --- UPDATE Operations ---');
    await assertCommandSuccess(`UPDATE inventory SET quantity = '450' WHERE product_id = 'prod1'`, 'Updates inventory quantity');
    await assertQueryResult(`SELECT quantity FROM inventory WHERE product_id = 'prod1'`, [{ "quantity": "450" }], 'Selects updated inventory quantity');

    console.log('\n  --- Advanced Queries ---');
    await assertQueryResult(`SELECT * FROM products WHERE unit_cost > 10`, [{ "description": "High-quality gadget", "name": "Product One", "product_id": "prod1", "supplier_id": "sup1", "unit_cost": "19.99" }], 'Finds products with unit_cost > 10');
    await assertQueryResult(`SELECT * FROM suppliers WHERE name LIKE 'Supplier%'`, [{ "contact_email": "contact@suppliera.com", "name": "Supplier A", "rating": "4", "supplier_id": "sup1" }], 'Finds suppliers with name LIKE "Supplier%"');
    
    await assertCommandSuccess(`INSERT INTO suppliers VALUES ('sup2', 'Supplier B', 'contact@supplierb.com', '5');`, 'Inserts a second supplier for IN test');
    await assertQueryResult(`SELECT * FROM suppliers WHERE name IN ('Supplier A', 'Supplier B')`, 
        [{ "contact_email": "contact@suppliera.com", "name": "Supplier A", "rating": "4", "supplier_id": "sup1" }, { "contact_email": "contact@supplierb.com", "name": "Supplier B", "rating": "5", "supplier_id": "sup2" }], 
        'Finds suppliers with name IN (...)');

    console.log('\n  --- DELETE Operations ---');
    await assertCommandSuccess(`DELETE FROM suppliers WHERE supplier_id = 'sup2'`, 'Deletes a supplier');
    await assertQueryResult(`SELECT * FROM suppliers WHERE supplier_id = 'sup2'`, [], 'Selects to confirm deletion');
}

async function main() {
  try {
    console.log('Assuming Go server is running in a separate terminal.');
    await new Promise(resolve => setTimeout(resolve, 1000));

    await setupDatabase();
    await runTests();

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
