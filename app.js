const http = require('http');
const readline = require('readline');

const dbName = '1';

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
          resolve(JSON.parse(data));
        } catch (e) {
          reject(`Failed to parse server response: ${data}`);
        }
      });
    });

    req.on('error', (e) => reject(`API request error: ${e.message}`));
    req.write(postData);
    req.end();
  });
}

// --- Database Setup ---
async function setupDatabase() {
  console.log('Initializing Supply Chain Management Database...');
  try {
    await sendQuery('', `CREATE DATABASE ${dbName}`);
    console.log(`Database '${dbName}' created.`);

    const schema = `
      CREATE TABLE suppliers (
          supplier_id VARCHAR(255),
          name VARCHAR(255),
          contact_email VARCHAR(255),
          rating INT
      );
      CREATE TABLE products (
          product_id VARCHAR(255),
          name VARCHAR(255),
          description VARCHAR(255),
          unit_cost FLOAT,
          supplier_id VARCHAR(255)
      );
      CREATE TABLE warehouses (
          warehouse_id VARCHAR(255),
          name VARCHAR(255),
          location VARCHAR(255),
          capacity INT
      );
      CREATE TABLE inventory (
          inventory_id VARCHAR(255),
          product_id VARCHAR(255),
          warehouse_id VARCHAR(255),
          quantity INT,
          last_updated VARCHAR(255)
      );
      CREATE TABLE shipments (
          shipment_id VARCHAR(255),
          product_id VARCHAR(255),
          from_warehouse_id VARCHAR(255),
          to_warehouse_id VARCHAR(255),
          quantity INT,
          status VARCHAR(100)
      );
    `;
    
    const createTableStatements = schema.split(';').filter(s => s.trim().length > 0);

    console.log('Creating tables...');
    for (const statement of createTableStatements) {
        const query = statement.trim() + ';';
        const res = await sendQuery(dbName, query);
        if (res.error) {
            // It might fail if the table already exists, which is okay on restart.
            if (!res.error.includes("already exists")) { // A hypothetical better error message
                 console.error(`Failed to execute: ${query}`, res.error);
            }
        } else {
            const tableNameMatch = query.match(/CREATE TABLE (\w+)/);
            if (tableNameMatch) {
                console.log(`- Table '${tableNameMatch[1]}' created.`);
            }
        }
    }

    console.log('Database setup complete.');
  } catch (error) {
    // This will catch errors from CREATE DATABASE if it already exists.
    console.log('Setup may have already been completed. Continuing...');
  }
}

// --- Helper for User Input ---
function askQuestion(rl, query) {
  return new Promise(resolve => rl.question(query, resolve));
}

function safeParseResult(res) {
    if (!res || !res.result || res.result === 'null') {
        return [];
    }
    try {
        return JSON.parse(res.result);
    } catch (e) {
        console.error("Failed to parse result:", res.result);
        return [];
    }
}

// --- Core Application Logic ---

async function handleAddSupplier(rl) {
    const id = await askQuestion(rl, 'Enter Supplier ID: ');
    const name = await askQuestion(rl, 'Enter Supplier Name: ');
    const email = await askQuestion(rl, 'Enter Contact Email: ');
    const rating = await askQuestion(rl, 'Enter Rating (1-5): ');
    const query = `INSERT INTO suppliers VALUES ('${id}', '${name}', '${email}', '${rating}');`;
    const res = await sendQuery(dbName, query);
    console.log('Response:', res.result || res.error);
}

async function handleAddProduct(rl) {
    const id = await askQuestion(rl, 'Enter Product ID: ');
    const name = await askQuestion(rl, 'Enter Product Name: ');
    const desc = await askQuestion(rl, 'Enter Description: ');
    const cost = await askQuestion(rl, 'Enter Unit Cost: ');
    const supplierId = await askQuestion(rl, 'Enter Supplier ID for this product: ');
    const query = `INSERT INTO products VALUES ('${id}', '${name}', '${desc}', '${cost}', '${supplierId}');`;
    const res = await sendQuery(dbName, query);
    console.log('Response:', res.result || res.error);
}

async function handleUpdateInventory(rl) {
    const productId = await askQuestion(rl, 'Enter Product ID: ');
    const warehouseId = await askQuestion(rl, 'Enter Warehouse ID: ');
    const quantity = await askQuestion(rl, 'Enter new quantity: ');
    const inventoryId = `inv_${productId}_${warehouseId}`;
    const date = new Date().toISOString();

    // This is a simplified "upsert". We try to update, if it fails, we insert.
    // A real wwfsdb might have a dedicated UPSERT or we'd have to query first.
    const updateQuery = `UPDATE inventory SET quantity = '${quantity}' WHERE inventory_id = '${inventoryId}'`;
    let res = await sendQuery(dbName, updateQuery);

    if (res.error && res.error.includes("no rows found to update")) {
        console.log('No existing inventory record. Creating new one...');
        const insertQuery = `INSERT INTO inventory VALUES ('${inventoryId}', '${productId}', '${warehouseId}', '${quantity}', '${date}');`;
        res = await sendQuery(dbName, insertQuery);
    }
    console.log('Response:', res.result || res.error);
}

async function handleCreateShipment(rl) {
    const id = `ship_${Date.now()}`;
    const productId = await askQuestion(rl, 'Enter Product ID: ');
    const fromId = await askQuestion(rl, 'Enter From Warehouse ID: ');
    const toId = await askQuestion(rl, 'Enter To Warehouse ID: ');
    const quantity = parseInt(await askQuestion(rl, 'Enter Quantity to ship: '), 10);

    // 1. Check if there is enough inventory
    const checkRes = await sendQuery(dbName, `SELECT * FROM inventory WHERE product_id = '${productId}' AND warehouse_id = '${fromId}'`);
    const inventoryItems = safeParseResult(checkRes);
    const sourceInventory = inventoryItems[0];

    if (!sourceInventory || parseInt(sourceInventory.quantity, 10) < quantity) {
        console.log(`Error: Not enough inventory. Available: ${sourceInventory ? sourceInventory.quantity : 0}`);
        return;
    }

    // 2. Create the shipment
    const shipmentQuery = `INSERT INTO shipments VALUES ('${id}', '${productId}', '${fromId}', '${toId}', '${quantity}', 'In-Transit');`;
    await sendQuery(dbName, shipmentQuery);

    // 3. Decrement inventory from source warehouse
    const newSourceQty = parseInt(sourceInventory.quantity, 10) - quantity;
    const updateSourceQuery = `UPDATE inventory SET quantity = '${newSourceQty}' WHERE inventory_id = '${sourceInventory.inventory_id}'`;
    await sendQuery(dbName, updateSourceQuery);

    console.log(`Shipment ${id} created. Inventory updated.`);
}

async function handleReceiveShipment(rl) {
    const shipmentId = await askQuestion(rl, 'Enter Shipment ID to mark as received: ');
    
    // 1. Find the shipment
    const shipRes = await sendQuery(dbName, `SELECT * FROM shipments WHERE shipment_id = '${shipmentId}'`);
    const shipments = safeParseResult(shipRes);
    const shipment = shipments[0];

    if (!shipment || shipment.status === 'Received') {
        console.log('Error: Shipment not found or already received.');
        return;
    }

    // 2. Update shipment status
    await sendQuery(dbName, `UPDATE shipments SET status = 'Received' WHERE shipment_id = '${shipmentId}'`);

    // 3. Increment inventory in destination warehouse (simplified upsert)
    const { product_id, to_warehouse_id, quantity } = shipment;
    const invId = `inv_${product_id}_${to_warehouse_id}`;
    const checkInvRes = await sendQuery(dbName, `SELECT * FROM inventory WHERE inventory_id = '${invId}'`);
    const destItems = safeParseResult(checkInvRes);
    const destInventory = destItems[0];

    if (destInventory) {
        const newQty = parseInt(destInventory.quantity, 10) + parseInt(quantity, 10);
        await sendQuery(dbName, `UPDATE inventory SET quantity = '${newQty}' WHERE inventory_id = '${invId}'`);
    } else {
        await sendQuery(dbName, `INSERT INTO inventory VALUES ('${invId}', '${product_id}', '${to_warehouse_id}', '${quantity}', '${new Date().toISOString()}');`);
    }
    
    console.log(`Shipment ${shipmentId} marked as received. Destination inventory updated.`);
}


async function viewInventoryReport() {
    console.log('\n--- Detailed Inventory Report ---');
    const invRes = await sendQuery(dbName, 'SELECT * FROM inventory');
    const inventory = safeParseResult(invRes);

    if (inventory.length === 0) {
        console.log('No inventory data found.');
        return;
    }

    // In a real scenario with joins, this would be one query.
    // Here, we demonstrate client-side joining.
    const prodRes = await sendQuery(dbName, 'SELECT * FROM products');
    const products = safeParseResult(prodRes).reduce((map, p) => (map[p.product_id] = p, map), {});
    
    const whRes = await sendQuery(dbName, 'SELECT * FROM warehouses');
    const warehouses = safeParseResult(whRes).reduce((map, w) => (map[w.warehouse_id] = w, map), {});

    const report = inventory.map(item => {
        const product = products[item.product_id] || { name: 'Unknown', unit_cost: 0 };
        const warehouse = warehouses[item.warehouse_id] || { name: 'Unknown' };
        return {
            'Warehouse': warehouse.name,
            'Product': product.name,
            'Quantity': item.quantity,
            'Unit Cost': parseFloat(product.unit_cost).toFixed(2),
            'Total Value': (parseInt(item.quantity, 10) * parseFloat(product.unit_cost)).toFixed(2),
        };
    });

    console.table(report);
}

async function listAll(tableName) {
    const res = await sendQuery(dbName, `SELECT * FROM ${tableName}`);
    const data = safeParseResult(res);
    if (data.length > 0) {
        console.table(data);
    } else {
        console.log(`No entries found in ${tableName}.`);
    }
}

// --- Main Application Loop ---
function showMenu() {
  console.log('\n--- Supply Chain Management System ---');
  console.log('1. Add Supplier');
  console.log('2. Add Product');
  console.log('3. Add/Update Inventory Stock');
  console.log('4. Create Shipment');
  console.log('5. Receive Shipment');
  console.log('---');
  console.log('6. View Detailed Inventory Report');
  console.log('7. List All Suppliers');
  console.log('8. List All Products');
  console.log('9. List All Shipments');
  console.log('---');
  console.log('10. Exit');
}

async function menuLoop(rl) {
  showMenu();
  const choice = await askQuestion(rl, 'Enter your choice: ');

  switch (choice) {
    case '1': await handleAddSupplier(rl); break;
    case '2': await handleAddProduct(rl); break;
    case '3': await handleUpdateInventory(rl); break;
    case '4': await handleCreateShipment(rl); break;
    case '5': await handleReceiveShipment(rl); break;
    case '6': await viewInventoryReport(); break;
    case '7': await listAll('suppliers'); break;
    case '8': await listAll('products'); break;
    case '9': await listAll('shipments'); break;
    case '10': rl.close(); return;
    default: console.log('Invalid choice. Please try again.');
  }

  await menuLoop(rl);
}

async function main() {
  await setupDatabase();

  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout
  });

  rl.on('close', () => {
    console.log('Exiting application.');
    process.exit(0);
  });

  await menuLoop(rl);
}

main();
