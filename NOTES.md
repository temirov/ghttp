# Notes

## Rules of engagement

Review the NOTES.md. Make a plan for autonomously fixing every item under Features, BugFixes, Improvements, Maintenance. Ensure no regressions. Ensure adding tests. Lean into integration tests. Fix every issue. Document the changes.

Fix issues one by one. 
1. Create a new git branch with descriptive name
2. Describe an issue through tests. Ensure that the tests are comprehensive and failing to begin with.
3. Fix the issue
4. Rerun the tests
5. Repeat 2-4 untill the issue is fixed and comprehensive tests are passing
6. Write a nice comprehensive commit message AFTER EACH issue is fixed and tested and covered with tests. 
Do not work on all issues at once. Work at one issue at a time sequntially. 

Remove an issue from the NOTES.md after the issue is fixed: 
1. New and existing tests are passing without regressions
2. Commit the changes and push to the remote.

Leave Features, BugFixes, Improvements, Maintenance sections empty when all fixes are implemented but don't delete the sections themselves.

## Issues

### Features

### Improvements

- [ ] [GH-11] add --browse flag which always renders a content of a folder and never starts rendering with either readme.md or index.html. Allow to render md and html files if they are clicked
- [ ] [GH-12] allow passing either html or md file to start serving it, e.g. `ghttp cat.html` shall have cat.html served on a default port

### BugFixes

### Maintenance
