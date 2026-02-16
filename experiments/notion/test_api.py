#!/usr/bin/env python3
"""
Notion API Experimentation Script
Tests current Notion API capabilities and documents changes
"""

import os
import json
import requests
from datetime import datetime
from urllib.parse import urlparse

# Load from .env
import sys
sys.path.insert(0, '/Users/admin/Project/msgvault')
from dotenv import load_dotenv
load_dotenv('/Users/admin/Project/msgvault/.env')

NOTION_TOKEN = os.getenv('NOTION_INTEGRATION_TOKEN')
NOTION_API = "https://api.notion.com/v1"
NOTION_VERSION = "2024-06-15"  # Latest documented stable version

class NotionExperiment:
    def __init__(self):
        self.session = requests.Session()
        self.session.headers.update({
            "Authorization": f"Bearer {NOTION_TOKEN}",
            "Notion-Version": NOTION_VERSION,
            "Content-Type": "application/json"
        })
        self.results = {}

    def log(self, title, msg=""):
        print(f"\n{'='*50}")
        print(f"[{datetime.now().strftime('%H:%M:%S')}] {title}")
        print(f"{'='*50}")
        if msg:
            print(msg)

    def request(self, method, endpoint, body=None):
        """Make API request and return response"""
        url = NOTION_API + endpoint

        print(f"\n📡 {method} {endpoint}")
        print(f"   URL: {url}")

        try:
            if method == "GET":
                resp = self.session.get(url)
            else:
                resp = self.session.post(url, json=body)

            print(f"   Status: {resp.status_code}")

            if resp.status_code not in (200, 201):
                print(f"   ❌ Error: {resp.text[:200]}")
                return None

            data = resp.json()
            print(f"   ✅ Response keys: {list(data.keys())}")
            return data
        except Exception as e:
            print(f"   ❌ Exception: {e}")
            return None

    def run(self):
        """Run all experiments"""
        self.log("NOTION API EXPERIMENTATION",
                f"Token: {NOTION_TOKEN[:20]}...\nVersion: {NOTION_VERSION}")

        # Test 1: Search for all pages
        self.test_search()

        # Test 2: Get page details
        self.test_page_details()

        # Test 3: Get page blocks
        self.test_blocks()

        # Test 4: Query database
        self.test_database()

        # Test 5: Check API version support
        self.test_api_versions()

        # Test 6: Check for new endpoints
        self.test_new_features()

        self.log("RESULTS SUMMARY")
        print(json.dumps(self.results, indent=2))

    def test_search(self):
        self.log("TEST 1: Search Pages")

        resp = self.request("POST", "/search", {
            "filter": {"property": "object", "value": "page"},
            "sort": {"direction": "descending", "timestamp": "last_edited_time"},
            "page_size": 5
        })

        if resp:
            pages = resp.get('results', [])
            self.results['search_found_pages'] = len(pages)
            print(f"\n✅ Found {len(pages)} pages:")

            for i, page in enumerate(pages):
                page_id = page.get('id', '?')
                created = page.get('created_time', '?')
                modified = page.get('last_edited_time', '?')
                print(f"   [{i}] {page_id[:12]}... | Created: {created} | Modified: {modified}")

                # Extract title if available
                props = page.get('properties', {})
                if 'title' in props:
                    title_prop = props['title']
                    if isinstance(title_prop, dict):
                        title_items = title_prop.get('title', [])
                        if title_items:
                            title_text = title_items[0].get('plain_text', 'N/A')
                            print(f"        Title: {title_text}")

            self.results['pages'] = [p.get('id') for p in pages[:3]]

    def test_page_details(self):
        self.log("TEST 2: Page Details")

        pages = self.results.get('pages', [])
        if not pages:
            print("No pages from previous test")
            return

        page_id = pages[0]
        resp = self.request("GET", f"/pages/{page_id}")

        if resp:
            self.results['page_response_keys'] = list(resp.keys())
            print(f"\n✅ Page response structure:")
            print(f"   Keys: {list(resp.keys())}")

            # Check for new fields
            new_fields = ['data_source', 'data_source_id', 'parent_data_source']
            for field in new_fields:
                if field in resp:
                    print(f"   ⭐ NEW FIELD: {field} = {resp[field]}")

    def test_blocks(self):
        self.log("TEST 3: Blocks (Page Content)")

        pages = self.results.get('pages', [])
        if not pages:
            print("No pages from previous test")
            return

        page_id = pages[0]
        resp = self.request("GET", f"/blocks/{page_id}/children", {"page_size": 10})

        if resp:
            blocks = resp.get('results', [])
            self.results['block_types_found'] = list(set(b.get('type') for b in blocks))

            print(f"\n✅ Found {len(blocks)} blocks:")
            print(f"   Block types: {self.results['block_types_found']}")

            for i, block in enumerate(blocks[:5]):
                block_type = block.get('type', '?')
                print(f"   [{i}] {block_type}")

                # Check for new block types
                if block_type in ['synced_database', 'database', 'database_id']:
                    print(f"       ⭐ New database block type!")

                # Show content for text blocks
                if block_type in ['paragraph', 'heading_1', 'heading_2', 'heading_3']:
                    try:
                        content = block.get(block_type, {})
                        if isinstance(content, dict):
                            rich_text = content.get('rich_text', [])
                            if rich_text:
                                text = rich_text[0].get('plain_text', '(empty)')
                                print(f"       └─ {text[:50]}")
                    except:
                        pass

    def test_database(self):
        self.log("TEST 4: Database Query")

        db_id = os.getenv('NOTION_ID_DATABASE')
        if not db_id:
            print("No database ID provided")
            return

        # Clean ID
        db_id = db_id.replace('-', '').replace('?', '')[:32]

        resp = self.request("POST", f"/databases/{db_id}/query", {
            "page_size": 5
        })

        if resp:
            self.results['database_query_keys'] = list(resp.keys())
            records = resp.get('results', [])
            print(f"\n✅ Database has {len(records)} visible records")

            # Check for new query features
            new_features = ['filter_by_config', 'sort_config', 'query_database_config']
            for feature in new_features:
                if feature in resp:
                    print(f"   ⭐ NEW FEATURE: {feature}")

    def test_api_versions(self):
        self.log("TEST 5: API Version Compatibility")

        versions_to_test = [
            "2024-06-15",  # Current
            "2025-09-03",  # New mentioned in docs
            "2024-04-15",  # Older
        ]

        print("\nTesting different API versions via /search:")
        for version in versions_to_test:
            headers = self.session.headers.copy()
            headers["Notion-Version"] = version

            try:
                resp = requests.post(
                    NOTION_API + "/search",
                    headers=headers,
                    json={"filter": {"property": "object", "value": "page"}},
                    timeout=5
                )
                status = "✅" if resp.status_code in (200, 400) else "❌"
                print(f"   {status} {version}: {resp.status_code}")

                if resp.status_code != 200:
                    error = resp.json().get('message', 'Unknown error')
                    print(f"       └─ Error: {error[:80]}")
            except Exception as e:
                print(f"   ❌ {version}: {str(e)[:80]}")

    def test_new_features(self):
        self.log("TEST 6: New/Changed Features")

        print("\n🔍 Checking for new features mentioned in 2025-2026 updates:")

        new_endpoints = [
            ("/pages/{id}/move", "POST"),      # Move page API
            ("/pages", "POST"),                # Query by config
            ("/databases", "GET"),             # List databases
        ]

        for endpoint, method in new_endpoints:
            # We can't test all without parameters, but document them
            print(f"   📌 {method} {endpoint} (not tested - requires parameters)")

        print("\n🔍 Known 2025+ changes:")
        changes = [
            "data_source_id in parent object (2025-09-03)",
            "Multi-source databases support",
            "Move page API for page placement",
            "Webhook improvements",
            "Better error messages",
        ]
        for change in changes:
            print(f"   ⭐ {change}")

if __name__ == "__main__":
    exp = NotionExperiment()
    exp.run()
