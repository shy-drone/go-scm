## File Check

- [x] content.go
  - [x] test pass
- [x] git.go
  - [x] test pass
- [x] gitee.go but ratelimit not check
  - [x] test pass
- [x] issue.go
  - [x] test pass
- [x] linker.go
  - [x] test pass
- [x] org.go
  - [x] test pass
- [x] pr.go
  - [x] test pass, but no check
- [x] repo.go
  - [x] test pass
- [x] review.go
  - [x] test pass, but no check
- [x] user.go
  - [x] test pass, but no check
- [ ] webhook.go
  - [ ] test pass


## webhook 特殊处理

因为 gitee 没有单独的 tag/branch create/delete hook, 而是有和 github 类似的 push_repo_create 的 push 事件, 所以将这类 push 事件特殊处理成 create/delete 事件

## Known Bug

`pr` and `tag` is still not trigger ci/cd
