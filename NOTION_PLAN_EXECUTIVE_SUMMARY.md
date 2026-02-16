# Notion Integration Implementation Plan - Executive Summary

**Status**: Ready for Review & Approval
**Created**: 2026-02-16
**Review Target**: Today

---

## 📌 What We've Done

### Phase 1: Research & Discovery ✅
- [x] Tested Notion API with your credentials
- [x] Identified API version changes (2025-09-03 is current)
- [x] Documented new data model features (multi-source databases)
- [x] Discovered new fields: `last_edited_by`, `is_locked`, `in_trash`
- [x] Mapped block types to markdown format

### Phase 2: Architecture Planning ✅
- [x] Created comprehensive master plan (NOTION_INTEGRATION_MASTER_PLAN.md)
- [x] Designed 10 sub-components with clear responsibilities
- [x] Defined dependency graph and execution order
- [x] Established code style consistency with msgvault
- [x] Created TDD testing strategy (unit → integration → E2E)

### Phase 3: Detailed Sub-Plans ✅
- [x] Component 1: Models (JSON type definitions)
- [x] Component 2: Rate Limiter (token bucket algorithm)
- [x] Component 3: Notion API Client (HTTP layer)
- [x] Component 4: Markdown Converter (blocks → markdown)
- [x] Documented remaining components (5-9) in master plan

---

## 🏗️ Implementation Architecture

### 10 Sub-Components (For Parallel Development)

```
PHASE 1: Foundation (Can test independently)
├─ 1. Models (notion/models.go)
     ├─ Types for Page, Block, RichText, etc.
     ├─ JSON unmarshaling from real API
     └─ 0 external dependencies

├─ 2. Rate Limiter (notion/ratelimit.go)
     ├─ Token bucket algorithm
     ├─ 3 req/sec enforcement
     └─ 0 external dependencies

├─ 3. OAuth Manager (notion/oauth.go)
     ├─ Token storage (~/.msgvault/tokens/)
     ├─ Support for internal integrations
     └─ Minimal file I/O dependency

PHASE 2: Core Integration (Per-component testing)
├─ 4. Notion API Client (notion/client.go)
     ├─ HTTP wrapper with retry logic
     ├─ Depends on: Models, RateLimiter
     └─ HTTP mock server for tests

├─ 5. Markdown Converter (notion/markdown.go)
     ├─ Blocks → markdown transformation
     ├─ All formatting types supported
     └─ Depends only on: Models

├─ 6. Discovery Service (notion/discovery.go)
     ├─ Page/database discovery via search API
     ├─ Pagination and traversal
     └─ Depends on: Client, Models

├─ 7. Content Fetcher (notion/content_fetcher.go)
     ├─ Retrieve blocks for pages/databases
     ├─ Handle recursive hierarchies
     └─ Depends on: Client, Models

PHASE 3: Integration Layer
├─ 8. Store Extensions (store/ updates)
     ├─ New tables for Notion content
     ├─ Message mapping schema
     └─ Depends on: existing Store

├─ 9. Notion Adapter (notion/adapter.go)
     ├─ High-level orchestration
     ├─ Full sync workflow
     └─ Depends on: all above

PHASE 4: End-to-End
├─ 10. Integration Tests (notion/integration_test.go)
      ├─ Real API with test workspace
      ├─ Verify sync quality
      └─ 100% test coverage

└─ 11. CLI Commands (cmd/msgvault/cmd/notion_sync.go)
       ├─ add-account notion
       ├─ sync-full notion
       ├─ sync-incremental notion
       └─ Integration with existing CLI
```

---

## 🧪 Testing Strategy (TDD)

### Three-Layer Test Pyramid

**Layer 1: Unit Tests**
- Each component tested in isolation
- All external dependencies mocked
- JSON fixtures from real Notion API
- Coverage: ≥90% per component
- Speed: <100ms per test
- Example: `TestConvertBlockToMarkdown_Paragraph`

**Layer 2: Integration Tests**
- 2-3 components working together
- Mock HTTP server for Notion API
- In-memory SQLite for store tests
- Speed: <1s per test
- Example: `TestDiscovery_FetchContent_Convert_Pipeline`

**Layer 3: E2E Tests**
- Real API with test workspace
- Narrow scope (10 test pages)
- Verify end result correctness
- Speed: 5-30s (acceptable)
- Example: `TestSync_RealWorkspace_VerifyMarkdown`

### Test Data Strategy

**Stage 1 (Unit)**: JSON fixtures from real API responses
```json
{
  "object": "page",
  "id": "abc123...",
  "type": "heading_1",
  "heading_1": {"rich_text": [...]},
  ...
}
```

**Stage 2 (Integration)**: Mock objects
```go
type mockAPI struct {
    pages map[string]*Page
    blocks map[string]*Block
}
```

**Stage 3 (E2E)**: Real credentials + test data
```
NOTION_INTEGRATION_TOKEN=ntn_...
NOTION_TEST_WORKSPACE_ID=synnada
```

---

## 📊 Execution Plan

### Timeline (Parallel Components)

**Phase 1: Foundation** (~3 hours)
- Models (2h) → Can start immediately
- Rate Limiter (1h) → Can start immediately
- OAuth Manager (1h) → Can start immediately
- **Parallelizable**: YES ✅

**Phase 2: Core Integration** (~8 hours)
- Notion Client (4h) - requires Models ✓, RateLimiter ✓
- Markdown Converter (3h) - requires Models ✓
- Discovery (2h) - requires Client ⏳
- Content Fetcher (2h) - requires Client ⏳
- **Parallelizable**: 50% (Client + Converter in parallel)

**Phase 3: Integration** (~4 hours)
- Store Extensions (2h) - blocks on Client ✓
- Adapter (2h) - blocks on everything above
- **Parallelizable**: NO (sequential)

**Phase 4: Testing & CLI** (~4 hours)
- Integration Tests (2h)
- CLI Commands (1h)
- Final verification (1h)
- **Parallelizable**: Partial

**Total**: ~19 hours, parallelizable 2-3 components at a time

---

## 💡 Key Design Decisions

### 1. **API Version**: Use v2025-09-03 (Latest)
- Supports multi-source databases
- New fields: `last_edited_by`, `is_locked`
- Same breaking change (not backward compatible with v2022-06-28)
- ✅ Decision: Use latest

### 2. **Markdown Format**: Plain Markdown (No HTML)
- Clean, searchable text
- Compatible with msgvault's FTS5
- Flexible for future rendering
- ✅ Decision: Simple markdown > HTML

### 3. **Notion Pages → Messages**: Reuse Message Table
- Don't create separate `notion_messages` table
- Map `notion_pages` to existing `messages` table
- Leverage existing FTS5, query engine, TUI
- ✅ Decision: Extend schema, reuse entities

### 4. **Token Storage**: File-based + Optional Encryption
- Similar to Gmail OAuth tokens
- `~/.msgvault/tokens/notion_workspace.json`
- Read-only operations (no risk from leaks)
- ✅ Decision: File-based, no additional encryption (MVP)

### 5. **Sync Direction**: Read-Only (No Deletions)
- Notion API doesn't support mass deletion
- msgvault is archival tool, not sync tool
- Users manually delete from Notion if needed
- ✅ Decision: Read-only integration

---

## ⚠️ Known Limitations & Future Work

### MVP (This Plan)
- ✅ Page discovery (search API)
- ✅ Block content retrieval
- ✅ Markdown conversion
- ✅ Incremental sync (timestamp-based)
- ✅ FTS5 search across Notion content
- ✅ Database query support

### Post-MVP (Future)
- ⏳ Webhook real-time sync
- ⏳ Database export to CSV
- ⏳ Rich media embedding
- ⏳ At-rest encryption
- ⏳ Synced database handling (external sources)

---

## 🎯 Success Criteria

### Functional
- ✅ Authentication works (token stored/retrieved)
- ✅ Discover all pages in test workspace
- ✅ Fetch all block types correctly
- ✅ Convert to readable markdown
- ✅ Store in msgvault without duplication
- ✅ Search works via FTS5
- ✅ Incremental sync detects changes

### Quality
- ✅ 90%+ test coverage per component
- ✅ Zero conflicts between components
- ✅ Code follows msgvault conventions
- ✅ Error handling comprehensive
- ✅ Performance comparable to Gmail sync

### Operational
- ✅ CLI commands work end-to-end
- ✅ Rate limiting respected
- ✅ Retry logic handles transient errors
- ✅ Real-workspace sync verified

---

## 📋 Sub-Agent Assignment

Each sub-agent will own their component end-to-end:

1. **Agent: ModelBuilder** → Component 1 (Models)
   - Deliverable: types.go with 100% test coverage
   - Review criteria: JSON parsing correctness, type completeness

2. **Agent: ClientBuilder** → Components 2 + 3 (RateLimiter + Client)
   - Deliverable: client.go with HTTP mock tests
   - Review criteria: Retry logic, error handling, rate limiting

3. **Agent: ConverterBuilder** → Component 4 (Markdown)
   - Deliverable: markdown.go with comprehensive fixtures
   - Review criteria: Formatting correctness, edge cases

4. **Agent: OAuthBuilder** → Component 5 (OAuth)
   - Deliverable: oauth.go with token lifecycle tests
   - Review criteria: Token storage, rotation, error cases

5. **Agent: DiscoveryBuilder** → Component 6 (Discovery)
   - Deliverable: discovery.go with pagination tests
   - Review criteria: Completeness, child page traversal

6. **Agent: FetcherBuilder** → Component 7 (ContentFetcher)
   - Deliverable: content_fetcher.go with block hierarchy tests
   - Review criteria: Recursive handling, performance

7. **Agent: StoreBuilder** → Component 8 (Store Extensions)
   - Deliverable: schema updates with data integrity tests
   - Review criteria: Query performance, data consistency

8. **Agent: AdapterBuilder** → Component 9 (Adapter)
   - Deliverable: adapter.go orchestrating full sync
   - Review criteria: Workflow correctness, error recovery

9. **Agent: IntegrationBuilder** → Components 10 + 11 (Tests + CLI)
   - Deliverable: integration tests + CLI commands
   - Review criteria: E2E correctness, CLI UX

---

## 🔄 Review & Integration Process

### Step 1: Component Review (Staggered)
As each agent completes their component:
- Review code quality & tests
- Verify no breaking style violations
- Check error handling completeness

### Step 2: Integration Review
Once 2+ components ready:
- Test components together
- Verify no API contract mismatches
- Resolve conflicts or assumptions

### Step 3: Dependency Chain Validation
Before adapters:
- Verify all dependencies available
- Test full data flow (end-to-end mock)
- Performance profiling

### Step 4: E2E Testing
Before release:
- Real API with test workspace
- Verify data integrity
- Performance benchmarking

---

## 📦 Deliverables

### Code
```
internal/notion/
├── models.go + _test.go
├── ratelimit.go + _test.go
├── client.go + _test.go
├── oauth.go + _test.go
├── discovery.go + _test.go
├── content_fetcher.go + _test.go
├── markdown.go + _test.go
├── adapter.go + _test.go
├── integration_test.go
├── mock.go + _test.go
├── testutil.go
└── testdata/
    ├── *.json (fixtures)
    └── README.md

cmd/msgvault/cmd/
├── notion_sync.go
└── notion_*.go (subcommands)

internal/store/
├── notion_schema.sql (new tables)
└── notion_store.go (extensions)
```

### Documentation
```
NOTION_INTEGRATION_MASTER_PLAN.md ✅ Done
NOTION_SUB_COMPONENT_PLANS.md ✅ Done
experiments/notion/API_CHANGES_FOUND.md ✅ Done
internal/notion/README.md (component guide)
examples/sync_notion.md (user guide)
```

### Tests
- Unit: 100+ tests (~500 LOC)
- Integration: 20+ tests
- E2E: 10+ tests
- Total coverage: 90%+

---

## ✅ Next Steps

### Option 1: Proceed with Sub-Agents (Recommended)
- [x] You approve the plan
- [ ] I create 9 sub-agents (one per major component)
- [ ] Sub-agents implement in parallel
- [ ] I review & integrate their outputs
- [ ] Final E2E verification
- **Timeline**: ~1 week with parallel execution

### Option 2: Adjust Plan First
- Clarify any ambiguities
- Add/remove components
- Adjust testing strategy
- Recalibrate timeline
**Timeline**: +1-2 days

### Option 3: Implement Sequentially (Classic)
- No parallelization
- One component at a time
- Full integration after each
**Timeline**: ~1 month

---

## ❓ Questions for Clarification

Before we proceed, please confirm:

1. **API Version**: Use v2025-09-03 (latest, breaks backward compat)?
   - ✅ Recommended: YES (get new features)

2. **Read-Only**: Pages retrieve-only, no deletion from Notion?
   - ✅ Recommended: YES (safer, aligns with archival use)

3. **Storage Model**: Map Notion pages to existing `messages` table?
   - ✅ Recommended: YES (reuse FTS5, query engine, TUI)

4. **Parallelization**: Launch 9 sub-agents simultaneously?
   - ✅ Recommended: YES (cut timeline from 1 month to 1 week)

5. **Test Workspace**: Can we use the `synnada` workspace from `.env`?
   - ✅ Recommended: YES (we already have test pages)

---

## 🎬 Approval Checklist

Please confirm before proceeding:

- [ ] Master plan is comprehensive and correct
- [ ] 10 sub-components are well-defined
- [ ] Testing strategy (TDD) is appropriate
- [ ] Code style conventions are clear
- [ ] Execution order makes sense
- [ ] You're ready for parallel sub-agent development
- [ ] Comfort level with this approach is HIGH

---

**Ready to launch sub-agents? 🚀**

Once you confirm the above, I will:
1. Create detailed implementation briefs for each sub-agent
2. Launch 9 sub-agents in parallel
3. Coordinate reviews and integration
4. Guide them through any blockers
5. Ensure quality and consistency across components
6. Final merge and verification

---

*Prepared by Claude Code | Estimated Review Time: 10-15 minutes*
