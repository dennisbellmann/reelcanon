CREATE SEQUENCE canon_draft_revision_seq;
ALTER TABLE canon_drafts ALTER COLUMN revision SET DEFAULT nextval('canon_draft_revision_seq');
SELECT setval('canon_draft_revision_seq', COALESCE((SELECT MAX(revision) FROM canon_drafts), 1));
