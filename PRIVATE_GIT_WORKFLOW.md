# Private Git Workflow

本目录是 Sub2API 私人 fork 的本地工作副本：

- Path: `D:\wflogin\sub2api-private`
- Working branch: `custom/kiro-windsurf-adapters`
- Official upstream: `https://github.com/Wei-Shaw/sub2api.git`
- Private origin: wait for your private Git repository URL

## Add Private Origin

After creating the private repository, add it as `origin`:

```powershell
cd D:\wflogin\sub2api-private
git remote add origin <your-private-repo-url>
git push -u origin custom/kiro-windsurf-adapters
```

If the private repository should use `main` as the default branch:

```powershell
git branch -M main
git push -u origin main
```

## Daily Commit Flow

```powershell
cd D:\wflogin\sub2api-private
git status --short
git add <changed-files>
git commit -m "type: short description"
git push
```

## Rollback Flow

View recent commits:

```powershell
git log --oneline --decorate -10
```

Rollback local files to a previous commit while keeping history:

```powershell
git revert <commit>
git push
```

Hard rollback only when explicitly intended:

```powershell
git reset --hard <commit>
git push --force-with-lease
```

Prefer `git revert` for server/deployment history because it is safer and
keeps an audit trail.

## Sync Official Sub2API

```powershell
cd D:\wflogin\sub2api-private
git fetch upstream main
git log --oneline HEAD..upstream/main
git merge upstream/main
git push
```

If there are conflicts, resolve them locally, run tests/build, then commit the
merge.

## Server Deployment Flow

For source-based custom deployment later:

1. Merge and test locally.
2. Push to the private repository.
3. Pull or build the same commit on the server.
4. Backup server data before replacing the running image/container.
5. Deploy and verify public Sub2API plus internal provider adapters.

Never commit tokens, account exports, `.env` files, server passwords, or private
provider credentials.
