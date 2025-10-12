document.addEventListener('DOMContentLoaded', () => {
    const usersList = document.getElementById('users-list');
    const nftsList = document.getElementById('nfts-list');
    const transactionsList = document.getElementById('transactions-list');
    const collectionsList = document.getElementById('collections-list');
    const categoriesList = document.getElementById('categories-list');
    const bidsList = document.getElementById('bids-list');
    const tagsList = document.getElementById('tags-list');
    const nftTagsList = document.getElementById('nft-tags-list');

    const createUserForm = document.getElementById('create-user-form');
    const createNftForm = document.getElementById('create-nft-form');
    const createCollectionForm = document.getElementById('create-collection-form');
    const createCategoryForm = document.getElementById('create-category-form');
    const createBidForm = document.getElementById('create-bid-form');
    const createTagForm = document.getElementById('create-tag-form');
    const createNftTagForm = document.getElementById('create-nft-tag-form');

    async function fetchData(endpoint, listElement) {
        try {
            const response = await fetch(`/api/${endpoint}`);
            const data = await response.json();
            listElement.innerHTML = '';
            if (data.length === 0) {
                listElement.innerHTML = '<p>No data found.</p>';
                return;
            }
            const table = document.createElement('table');
            const thead = document.createElement('thead');
            const tbody = document.createElement('tbody');
            const headers = Object.keys(data[0]);
            const headerRow = document.createElement('tr');
            headers.forEach(header => {
                const th = document.createElement('th');
                th.textContent = header;
                headerRow.appendChild(th);
            });
            thead.appendChild(headerRow);
            table.appendChild(thead);
            data.forEach(item => {
                const row = document.createElement('tr');
                headers.forEach(header => {
                    const cell = document.createElement('td');
                    cell.textContent = item[header];
                    row.appendChild(cell);
                });
                tbody.appendChild(row);
            });
            table.appendChild(tbody);
            listElement.appendChild(table);
        } catch (error) {
            listElement.innerHTML = `<p>Error fetching data: ${error.message}</p>`;
        }
    }

    async function createData(endpoint, body) {
        try {
            const response = await fetch(`/api/${endpoint}`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(body),
            });
            const result = await response.json();
            console.log(result);
            // Refresh all data after creation
            fetchAllData();
        } catch (error) {
            console.error(`Error creating data: ${error.message}`);
        }
    }

    createUserForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const id = document.getElementById('user-id').value;
        const username = document.getElementById('username').value;
        const wallet_address = document.getElementById('wallet-address').value;
        const created_at = document.getElementById('user-created-at').value;
        const updated_at = document.getElementById('user-updated-at').value;
        const profile_image_url = document.getElementById('user-profile-image-url').value;
        const bio = document.getElementById('user-bio').value;
        createData('users', { id: parseInt(id), username, wallet_address, created_at, updated_at, profile_image_url, bio });
        createUserForm.reset();
    });

    createCollectionForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const id = document.getElementById('collection-id').value;
        const name = document.getElementById('collection-name').value;
        const description = document.getElementById('collection-description').value;
        const creator_id = document.getElementById('collection-creator-id').value;
        createData('collections', { id: parseInt(id), name, description, creator_id: parseInt(creator_id) });
        createCollectionForm.reset();
    });

    createCategoryForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const id = document.getElementById('category-id').value;
        const name = document.getElementById('category-name').value;
        createData('categories', { id: parseInt(id), name });
        createCategoryForm.reset();
    });

    createNftForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const id = document.getElementById('nft-id').value;
        const name = document.getElementById('nft-name').value;
        const description = document.getElementById('nft-description').value;
        const image_url = document.getElementById('nft-image-url').value;
        const owner_id = document.getElementById('nft-owner-id').value;
        const creator_id = document.getElementById('nft-creator-id').value;
        const price = document.getElementById('nft-price').value;
        const created_at = document.getElementById('nft-created-at').value;
        const updated_at = document.getElementById('nft-updated-at').value;
        const collection_id = document.getElementById('nft-collection-id').value;
        const category_id = document.getElementById('nft-category-id').value;
        createData('nfts', { id: parseInt(id), name, description, image_url, owner_id: parseInt(owner_id), creator_id: parseInt(creator_id), price: parseFloat(price), created_at, updated_at, collection_id: parseInt(collection_id), category_id: parseInt(category_id) });
        createNftForm.reset();
    });

    createBidForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const id = document.getElementById('bid-id').value;
        const nft_id = document.getElementById('bid-nft-id').value;
        const bidder_id = document.getElementById('bidder-id').value;
        const bid_amount = document.getElementById('bid-amount').value;
        const bid_date = document.getElementById('bid-date').value;
        createData('bids', { id: parseInt(id), nft_id: parseInt(nft_id), bidder_id: parseInt(bidder_id), bid_amount: parseFloat(bid_amount), bid_date });
        createBidForm.reset();
    });

    createTagForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const id = document.getElementById('tag-id').value;
        const name = document.getElementById('tag-name').value;
        createData('tags', { id: parseInt(id), name });
        createTagForm.reset();
    });

    createNftTagForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const nft_id = document.getElementById('nft-tag-nft-id').value;
        const tag_id = document.getElementById('nft-tag-tag-id').value;
        createData('nft_tags', { nft_id: parseInt(nft_id), tag_id: parseInt(tag_id) });
        createNftTagForm.reset();
    });

    function fetchAllData() {
        fetchData('users', usersList);
        fetchData('nfts', nftsList);
        fetchData('transactions', transactionsList);
        fetchData('collections', collectionsList);
        fetchData('categories', categoriesList);
        fetchData('bids', bidsList);
        fetchData('tags', tagsList);
        fetchData('nft_tags', nftTagsList);
    }

    // Initial data fetch
    fetchAllData();
});