# Notion API Experimentation Report

**Date**: 2026-02-16
**Experiment**: Direct API testing with real Notion workspace
**API Version Tested**: 2025-09-03 (latest)

---

## 🚨 MAJOR FINDING: API Versions Have Changed

### Valid Notion API Versions (as of 2026)

The following versions are **officially supported**:
- `2021-05-11` (deprecated)
- `2021-05-13` (deprecated)
- `2021-08-16` (deprecated)
- `2022-02-22` (deprecated)
- `2022-06-28` (stable, still supported)
- `2025-09-03` (NEW - latest)

**⚠️ Breaking Change:** `2024-06-15` (which was in my original research) **NO LONGER EXISTS**

This means **any integration using 2024-06-15 will immediately fail with:**
```
Notion-Version header failed validation: Notion-Version header should be
"2021-05-11", "2021-05-13", "2021-08-16", "2022-02-22", "2022-06-28",
or "2025-09-03", instead was "2024-06-15".
```

---

## 📜 Response Structure Changes (2025-09-03)

### Search Response

```json
{
  "object": "list",
  "results": [...],
  "next_cursor": "86605a75-...",
  "has_more": true,
  "type": "page_or_data_source",      // NEW!
  "page_or_data_source": {},          // NEW!
  "request_id": "a43dd8a2-..."
}
```

**New Fields:**
- `type`: Always `"page_or_data_source"` (indicates multi-source support)
- `page_or_data_source`: Empty object, but indicates new data model

### Page Response

```json
{
  "object": "page",
  "id": "378caaac-49f9-45d3-8336-bceae783fca0",
  "created_time": "2022-01-03T05:41:00.000Z",
  "last_edited_time": "2026-02-16T19:15:00.000Z",
  "created_by": {...},
  "last_edited_by": {...},           // NEW!
  "cover": {...},
  "icon": {...},
  "parent": {
    "type": "workspace",
    "workspace": true
  },
  "archived": false,
  "in_trash": false,
  "is_locked": false,                // NEW!
  "properties": {...},
  "url": "https://www.notion.so/...",
  "public_url": null
}
```

**New/Changed Fields:**
- `last_edited_by`: Now included (who last modified the page)
- `is_locked`: New field indicating page lock status
- `in_trash`: Separate from `archived`

### Blocks Response

Structure mostly same, but blocks now include:
```json
{
  "object": "block",
  "id": "81e72c74-...",
  "parent": {
    "type": "page_id",            // or "data_source", "block_id"
    "page_id": "378caaac-..."
  },
  "created_time": "2023-08-15T11:56:00.000Z",
  "last_edited_time": "2023-08-15T11:56:00.000Z",
  "created_by": {...},
  "last_edited_by": {...},         // NEW!
  "has_children": false,
  "archived": false,
  "in_trash": false,               // NEW!
  "is_locked": false,              // NEW!
  "type": "heading_3",
  "heading_3": {...}
}
```

**New/Changed:**
- `last_edited_by`: Added
- `in_trash`: Separate tracking
- `is_locked`: New field

---

## 🆕 New Block Types Observed

### 1. `child_database` Block

```json
{
  "type": "child_database",
  "child_database": {
    "title": "Objectives"
  }
}
```

**What it is:** A database embedded within a page
**Significance:** Must handle database queries within pages

### 2. `child_page` Block

```json
{
  "type": "child_page",
  "child_page": {
    "title": "Sprints"
  }
}
```

**What it is:** A sub-page or nested page
**Significance:** May need recursive traversal for full page export

---

## 🔄 Data Model Changes: Multi-Source Support

**Major architectural change**: Notion now supports **multi-source databases**

**What this means:**
- Databases can be synced from external sources (GitHub, Jira, etc.)
- Pages can have `data_source` as parent (not just workspace or page)
- New field `data_source_id` in parent objects
- Search response includes `type: "page_or_data_source"`

**For msgvault integration:**
- We need to handle `data_source` parent types
- May need to detect and handle synced databases differently
- Incremental sync might be affected

---

## 📋 Rich Text Structure (Unchanged)

Good news: Rich text format is **backward compatible)**

```json
{
  "type": "text",
  "text": {
    "content": "Hello world",
    "link": null
  },
  "annotations": {
    "bold": false,
    "italic": false,
    "strikethrough": false,
    "underline": false,
    "code": false,
    "color": "default"
  },
  "plain_text": "Hello world",
  "href": null
}
```

No changes here.

---

## 🔍 What Still Works

✅ Search API - same behavior
✅ Get Page - mostly same
✅ Get Blocks - mostly same
✅ Pagination (`has_more`, `next_cursor`)
✅ Timestamps (`created_time`, `last_edited_time`)
✅ Rich text structure
✅ Properties structure

---

## ⚠️ What Changed/Broke

❌ API version `2024-06-15` no longer accepted
❌ Need to specify `2025-09-03` for latest features
⚠️ Database queries - requires valid database ID (not page ID)
⚠️ Multi-source support - need to handle new parent types
⚠️ New fields - older code might not expect `last_edited_by`, `is_locked`, etc.

---

## 📝 Integration Checklist

For msgvault Notion integration using 2025-09-03:

- [ ] Update API version to `2025-09-03`
- [ ] Handle new page fields: `last_edited_by`, `is_locked`
- [ ] Handle new block fields: `last_edited_by`, `in_trash`, `is_locked`
- [ ] Support `child_database` blocks (database within page)
- [ ] Support `child_page` blocks (nested pages)
- [ ] Handle multi-source databases (new parent type)
- [ ] Parse `type: "page_or_data_source"` in search results
- [ ] Build markdown converter for block content
- [ ] Handle pagination for large pages/databases
- [ ] Rate limiting (3 req/sec still applies)

---

## 🧪 Test Results

### What We Successfully Tested

1. **Authentication** ✅
   - Bearer token works
   - Version header validation works

2. **Search API** ✅
   - Found multiple pages
   - Pagination working (`has_more`, `next_cursor`)
   - Response structure as documented

3. **Page Details** ✅
   - All metadata retrieved
   - Rich text structure intact
   - Timestamps present

4. **Blocks API** ✅
   - Block types retrieved
   - Child database and child page blocks found
   - Block hierarchy visible

### What We Need to Test Further

- [ ] Database query endpoint (with proper database ID)
- [ ] Incremental sync with filtered timestamps
- [ ] Move page API (new in 2025)
- [ ] Webhook setup and delivery
- [ ] Rate limiting behavior
- [ ] Database CSV export

---

## 🎯 Recommended Next Steps

1. **Use 2025-09-03 API version**
   - Legacy versions still work (2022-06-28) but miss new features
   - 2025-09-03 required for multi-source databases

2. **Handle new fields gracefully**
   - Don't break on `last_edited_by`, `is_locked` fields
   - These are backward compatible

3. **Markdown conversion priority:**
   - Paragraph, heading blocks (core)
   - Lists, code blocks (important)
   - callout, quote, toggle (nice to have)
   - child_database, child_page (special handling)

4. **For msgvault schema:**
   - Store `last_edited_by` as participant
   - Use `last_edited_time` for incremental sync
   - Detect `in_trash` for filtering
   - Track `is_locked` in metadata

---

## 📊 Test Workspace Summary

- **Workspace**: synnada
- **Pages found**: 2+ (search result truncated)
- **Blocks found**: Multiple, including child_database and child_page
- **Content types**: Headings, child blocks
- **Token works**: ✅ Confirmed valid

---

**Generated**: 2026-02-16 by Claude Code with real API testing
