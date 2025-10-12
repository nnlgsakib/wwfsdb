const express = require('express');
const router = express.Router();
const wwfsdb = require('../services/wwfsdb');

// Users
router.get('/users', async (req, res) => {
  try {
    const users = await wwfsdb.executeQuery('SELECT * FROM users');
    res.json(users);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

router.get('/users/:id', async (req, res) => {
  try {
    const user = await wwfsdb.executeQuery(`SELECT * FROM users WHERE id = ${req.params.id}`);
    res.json(user);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

router.post('/users', async (req, res) => {
    try {
        const { id, username, wallet_address, created_at, updated_at, profile_image_url, bio } = req.body;
        const result = await wwfsdb.executeWriteQuery(`INSERT INTO users (id, username, wallet_address, created_at, updated_at, profile_image_url, bio) VALUES (${id}, '${username}', '${wallet_address}', '${created_at}', '${updated_at}', '${profile_image_url}', '${bio}')`);
        res.json(result);
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});

// Collections
router.get('/collections', async (req, res) => {
    try {
        const collections = await wwfsdb.executeQuery('SELECT * FROM collections');
        res.json(collections);
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});

router.post('/collections', async (req, res) => {
    try {
        const { id, name, description, creator_id } = req.body;
        const result = await wwfsdb.executeWriteQuery(`INSERT INTO collections (id, name, description, creator_id) VALUES (${id}, '${name}', '${description}', ${creator_id})`);
        res.json(result);
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});

// Categories
router.get('/categories', async (req, res) => {
    try {
        const categories = await wwfsdb.executeQuery('SELECT * FROM categories');
        res.json(categories);
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});

router.post('/categories', async (req, res) => {
    try {
        const { id, name } = req.body;
        const result = await wwfsdb.executeWriteQuery(`INSERT INTO categories (id, name) VALUES (${id}, '${name}')`);
        res.json(result);
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});


// NFTs
router.get('/nfts', async (req, res) => {
    try {
        const nfts = await wwfsdb.executeQuery('SELECT * FROM nfts');
        res.json(nfts);
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});

router.get('/nfts/:id', async (req, res) => {
    try {
        const nft = await wwfsdb.executeQuery(`SELECT * FROM nfts WHERE id = ${req.params.id}`);
        res.json(nft);
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});

router.post('/nfts', async (req, res) => {
    try {
        const { id, name, description, image_url, owner_id, creator_id, price, created_at, updated_at, collection_id, category_id } = req.body;
        const result = await wwfsdb.executeWriteQuery(`INSERT INTO nfts (id, name, description, image_url, owner_id, creator_id, price, created_at, updated_at, collection_id, category_id) VALUES (${id}, '${name}', '${description}', '${image_url}', ${owner_id}, ${creator_id}, ${price}, '${created_at}', '${updated_at}', ${collection_id}, ${category_id})`);
        res.json(result);
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});

// Transactions
router.get('/transactions', async (req, res) => {
    try {
        const transactions = await wwfsdb.executeQuery('SELECT * FROM transactions');
        res.json(transactions);
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});

// Bids
router.get('/bids', async (req, res) => {
    try {
        const bids = await wwfsdb.executeQuery('SELECT * FROM bids');
        res.json(bids);
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});

router.post('/bids', async (req, res) => {
    try {
        const { id, nft_id, bidder_id, bid_amount, bid_date } = req.body;
        const result = await wwfsdb.executeWriteQuery(`INSERT INTO bids (id, nft_id, bidder_id, bid_amount, bid_date) VALUES (${id}, ${nft_id}, ${bidder_id}, ${bid_amount}, '${bid_date}')`);
        res.json(result);
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});

// Tags
router.get('/tags', async (req, res) => {
    try {
        const tags = await wwfsdb.executeQuery('SELECT * FROM tags');
        res.json(tags);
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});

router.post('/tags', async (req, res) => {
    try {
        const { id, name } = req.body;
        const result = await wwfsdb.executeWriteQuery(`INSERT INTO tags (id, name) VALUES (${id}, '${name}')`);
        res.json(result);
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});

// NFT Tags
router.get('/nft_tags', async (req, res) => {
    try {
        const nft_tags = await wwfsdb.executeQuery('SELECT * FROM nft_tags');
        res.json(nft_tags);
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});

router.post('/nft_tags', async (req, res) => {
    try {
        const { nft_id, tag_id } = req.body;
        const result = await wwfsdb.executeWriteQuery(`INSERT INTO nft_tags (nft_id, tag_id) VALUES (${nft_id}, ${tag_id})`);
        res.json(result);
    } catch (error) {
        res.status(500).json({ error: error.message });
    }
});


module.exports = router;
