-- Users table
CREATE TABLE users (
    id INT,
    username VARCHAR(255),
    wallet_address VARCHAR(255),
    created_at VARCHAR(255),
    updated_at VARCHAR(255),
    profile_image_url VARCHAR(255),
    bio VARCHAR(255)
);

-- Collections table
CREATE TABLE collections (
    id INT,
    name VARCHAR(255),
    description VARCHAR(255),
    creator_id INT -- Foreign key to users(id)
);

-- Categories table
CREATE TABLE categories (
    id INT,
    name VARCHAR(255)
);

-- NFTs table
CREATE TABLE nfts (
    id INT,
    name VARCHAR(255),
    description VARCHAR(255),
    image_url VARCHAR(255),
    owner_id INT, -- Foreign key to users(id)
    creator_id INT, -- Foreign key to users(id)
    price FLOAT,
    created_at VARCHAR(255),
    updated_at VARCHAR(255),
    collection_id INT, -- Foreign key to collections(id)
    category_id INT -- Foreign key to categories(id)
);

-- Transactions table
CREATE TABLE transactions (
    id INT,
    nft_id INT, -- Foreign key to nfts(id)
    buyer_id INT, -- Foreign key to users(id)
    seller_id INT, -- Foreign key to users(id)
    transaction_date VARCHAR(255),
    price FLOAT
);

-- Bids table
CREATE TABLE bids (
    id INT,
    nft_id INT, -- Foreign key to nfts(id)
    bidder_id INT, -- Foreign key to users(id)
    bid_amount FLOAT,
    bid_date VARCHAR(255)
);

-- Tags table
CREATE TABLE tags (
    id INT,
    name VARCHAR(255)
);

-- Join table for NFTs and Tags
CREATE TABLE nft_tags (
    nft_id INT, -- Foreign key to nfts(id)
    tag_id INT -- Foreign key to tags(id)
);

-- Indexes
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_nfts_owner_id ON nfts(owner_id);
CREATE INDEX idx_nfts_collection_id ON nfts(collection_id);
CREATE INDEX idx_transactions_nft_id ON transactions(nft_id);
CREATE INDEX idx_bids_nft_id ON bids(nft_id);