# Implementation Plan: Domain-Specific UIs with Framework Aggregation

**Spec**: 009-domain-ui-architecture  
**Status**: Phase 1 Design Ready  
**Timeline**: 8 weeks (parallel with backend after spec 008 Phase 3)

## Constitution Check ✅

**Principle III**: Domain UIs are static artefacts in `packs/{domain}/ui/`, framework serves  
**Principle VI**: Domain isolation via API contracts, not procedures

## Implementation Phases

**Phase 1**: Domain UI foundation (weeks 1-3) - structure, API binding, list/editor components  
**Phase 2**: Source integration (weeks 3-4) - status display, file upload, job triggers  
**Phase 3**: Rule management (weeks 4-5) - rule display, editing, conflict resolution  
**Phase 4**: Framework aggregation (weeks 5-7) - domain discovery, embedding, dashboard  
**Phase 5**: Testing (weeks 7-8) - end-to-end validation, performance optimization

## Parallelization

**Can run in parallel with spec 008 after Phase 3** (expense-domain restructured):
- Domain UI structure and components
- API client library
- Framework aggregation skeleton

## Success Metrics

✅ All domains have functional UIs  
✅ Framework aggregation loads <2 seconds  
✅ File uploads trigger jobs in <1 second  
✅ Rule management with zero data loss  
✅ Performance targets met (100ms rule load, 5s metric updates)

