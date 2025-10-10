const http = require('http');
const crypto = require('crypto');

const dbName = 'asset_marketplace';
const privateKey = 'b691c4637be697542d54a73555136d2b5f4864eed8820f5f7c916acd08b6fc9c';

const migrationState = {
    completed: 0,
    failed: 0,
    requestId: 1,
};

// --- Signing function (from app.js) ---
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

// --- Core API Communication (JSON-RPC) (from app.js) ---
async function sendQuery(dbName, query, sessionID = null, privateKey = null) {
    return new Promise((resolve, reject) => {
        const params = {
            db_name: dbName,
            query: query,
        };
        if (sessionID) {
            params.session_id = sessionID;
        }

        // Add signature for write queries if private key is provided
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
            id: migrationState.requestId++,
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

async function executeMigrationStep(query, description) {
    try {
        const res = await sendQuery(dbName, query, null, privateKey);
        if (res.error) {
            console.log(`  ❌ ${description}`);
            console.log(`     Error: ${res.error.message || res.error}`);
            migrationState.failed++;
            return false;
        } else {
            console.log(`  ✅ ${description}`);
            migrationState.completed++;
            return true;
        }
    } catch (error) {
        console.log(`  ❌ ${description}`);
        console.log(`     Error: ${error}`);
        migrationState.failed++;
        return false;
    }
}

async function runMigration() {
    console.log(`\n--- 🚀 Starting Migration for database: ${dbName} ---\n`);

    console.log(`Using database: ${dbName}`);
    console.log(`Using private key: ${privateKey}\n`);

    // SQL statements from ssql_example.ssql without comments inside statements
    const schemaStatements = [
        // Users table to store information about creators and customers
        `CREATE TABLE users (
    user_id VARCHAR(255),
    username VARCHAR(100),
    email VARCHAR(255),
    password_hash VARCHAR(255),
    created_at INT,
    is_verified BOOLEAN,
    bio VARCHAR(1000)
);`,

        // Asset types to categorize the digital goods (e.g., 3D Model, Texture, Audio)
        `CREATE TABLE asset_types (
    type_id VARCHAR(255),
    type_name VARCHAR(100),
    description VARCHAR(500)
);`,

        // The main table for all digital assets
        `CREATE TABLE assets (
    asset_id VARCHAR(255),
    creator_id VARCHAR(255),
    type_id VARCHAR(255),
    name VARCHAR(255),
    description VARCHAR(4000),
    base_price FLOAT,
    upload_date INT,
    last_updated INT,
    file_url VARCHAR(500),
    thumbnail_url VARCHAR(500),
    is_published BOOLEAN,
    poly_count INT,
    file_size_kb INT
);`,

        // Tags for assets to allow for easy searching and filtering
        `CREATE TABLE tags (
    tag_id VARCHAR(255),
    tag_name VARCHAR(100)
);`,

        // Junction table for the many-to-many relationship between assets and tags
        `CREATE TABLE asset_tags (
    asset_id VARCHAR(255),
    tag_id VARCHAR(255)
);`,

        // User reviews and ratings for each asset
        `CREATE TABLE reviews (
    review_id VARCHAR(255),
    asset_id VARCHAR(255),
    user_id VARCHAR(255),
    rating INT,
    comment VARCHAR(2000),
    created_at INT
);`,

        // Different license types available for purchase (e.g., Standard, Extended)
        `CREATE TABLE licenses (
    license_id VARCHAR(255),
    license_name VARCHAR(100),
    description VARCHAR(2000)
);`,

        // Links assets to available licenses and defines price adjustments
        `CREATE TABLE asset_licenses (
    asset_license_id VARCHAR(255),
    asset_id VARCHAR(255),
    license_id VARCHAR(255),
    price_multiplier FLOAT
);`,

        // Records of customer purchases
        `CREATE TABLE orders (
    order_id VARCHAR(255),
    user_id VARCHAR(255),
    order_date INT,
    total_amount FLOAT
);`,

        // Details of each item within an order
        `CREATE TABLE order_items (
    order_item_id VARCHAR(255),
    order_id VARCHAR(255),
    asset_license_id VARCHAR(255),
    purchase_price FLOAT
);`,

        // User-curated collections of assets (e.g., "Sci-Fi Assets", "My Favorites")
        `CREATE TABLE collections (
    collection_id VARCHAR(255),
    user_id VARCHAR(255),
    name VARCHAR(255),
    description VARCHAR(1000),
    is_public BOOLEAN,
    created_at INT
);`,

        // Junction table for the many-to-many relationship between collections and assets
        `CREATE TABLE collection_items (
    collection_id VARCHAR(255),
    asset_id VARCHAR(255)
);`,

        // Create indexes for frequently queried columns to improve performance
        `CREATE INDEX ON assets (creator_id);`,
        `CREATE INDEX ON assets (type_id);`,
        `CREATE INDEX ON asset_tags (asset_id);`,
        `CREATE INDEX ON asset_tags (tag_id);`,
        `CREATE INDEX ON reviews (asset_id);`,
        `CREATE INDEX ON orders (user_id);`
    ];

    console.log(`Found ${schemaStatements.length} SQL statements to execute.\n`);

    let stepNumber = 1;
    for (const statement of schemaStatements) {
        // Skip comments and empty lines
        if (statement.startsWith('--') || statement.startsWith('/*')) {
            continue;
        }
        
        // Extract a meaningful description from the statement
        let description = `Step ${stepNumber}: ${statement.substring(0, Math.min(60, statement.indexOf(' ')))}...`;
        if (statement.toUpperCase().startsWith('CREATE TABLE')) {
            const tableNameMatch = statement.match(/CREATE TABLE (\w+)/i);
            if (tableNameMatch) {
                const tableName = tableNameMatch[1];
                description = `Step ${stepNumber}: Creating table "${tableName}"`;
            }
        } else if (statement.toUpperCase().startsWith('CREATE INDEX')) {
            description = `Step ${stepNumber}: Creating index`;
        }
        
        await executeMigrationStep(statement, description);
        stepNumber++;
    }

    console.log('\n--- 🏁 Migration Completed ---');
    console.log(`    Total operations: ${migrationState.completed + migrationState.failed}`);
    console.log(`    ✅ Completed:    ${migrationState.completed}`);
    console.log(`    ❌ Failed:       ${migrationState.failed}`);
    console.log('---------------------------');
    
    return migrationState.failed === 0;
}

async function main() {
    console.log('Assuming Go server is running in a separate terminal.');
    await new Promise(resolve => setTimeout(resolve, 1000));

    try {
        const success = await runMigration();
        console.log('\n--- 🏁 Migration Process Finished ---');
        process.exit(success ? 0 : 1);
    } catch (error) {
        console.error('\n--- ❌ Migration Process Failed ---');
        console.error(error);
        process.exit(1);
    }
}

main();