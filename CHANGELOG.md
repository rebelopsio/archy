# Changelog

## [1.1.0](https://github.com/rebelopsio/archy/compare/v1.0.0...v1.1.0) (2026-05-13)


### Features

* add --version flag to root command ([e913838](https://github.com/rebelopsio/archy/commit/e91383857b790e342661785223048271da07d48c))
* add --version flag to root command ([b4f2d36](https://github.com/rebelopsio/archy/commit/b4f2d3658384f374fb992b0910bd6389eb293490))
* **config:** add UserConfig schema with validation and env binding ([9ea189a](https://github.com/rebelopsio/archy/commit/9ea189a23ee531c5e42506102ec57f23a0b0d334))
* **domain:** add Identity type for cross-provider operator identification ([624d267](https://github.com/rebelopsio/archy/commit/624d2679541df830f51f7274412d4454379ec4bb))
* **user-identity:** config-driven operator identity throughout data path ([d761405](https://github.com/rebelopsio/archy/commit/d761405ef5e11ef171dc581ec2d2bd22f2036e7b))


### Bug Fixes

* **agent:** pass --verbose to claude subprocess ([b127f5c](https://github.com/rebelopsio/archy/commit/b127f5c845fcf421a312ddd4a2b05c4130ea842d))
* **agent:** pass --verbose to claude subprocess ([274989a](https://github.com/rebelopsio/archy/commit/274989a27b92a37d2dbfc11fabde9be6bb054a10))
* **config:** honor XDG_CONFIG_HOME and ~/.config on macOS ([8576395](https://github.com/rebelopsio/archy/commit/85763954dd514654f7c8b642660fbae310146d59))
* **config:** honor XDG_CONFIG_HOME and ~/.config on macOS ([428df31](https://github.com/rebelopsio/archy/commit/428df3142f8ade85a07d5af15e584902bcfd794b))
* **daily:** include agent text in verification-failure error ([4f1abc3](https://github.com/rebelopsio/archy/commit/4f1abc3af22852f10e4b0e82b86b4dc34ef94f17))
* **daily:** include agent text in verification-failure error ([c87b0b0](https://github.com/rebelopsio/archy/commit/c87b0b03cc46013fec5ea1b4384ccf9e416f6c12))
* **daily:** surface turns, duration, cost in verification-failure error ([ec7ceaa](https://github.com/rebelopsio/archy/commit/ec7ceaa8721c28305eb442413b05254793e2aa5a))
* **daily:** surface turns, duration, cost in verification-failure error ([e265fa4](https://github.com/rebelopsio/archy/commit/e265fa40987f76ce73e507519ba03e1648972c4c))
* **daily:** take idempotency claim after success, not before ([00be35f](https://github.com/rebelopsio/archy/commit/00be35f6202fb1ec39f5de4248b659866ef903f2))
* **daily:** verify file exists after agent run ([af98715](https://github.com/rebelopsio/archy/commit/af987154416d9e3894d24cbd120761ec77e44e2e))
* **daily:** verify file exists after agent run ([7dc01a7](https://github.com/rebelopsio/archy/commit/7dc01a7aeb6e7b694c417a3d0b2b3ee0b6a1ba62))
* **linear:** assignee is a string, not an object ([d4078c5](https://github.com/rebelopsio/archy/commit/d4078c570a58e56fb526d45dbd14b7cc81d2a3e0))
* **linear:** assignee is a string, not an object ([bf5c936](https://github.com/rebelopsio/archy/commit/bf5c936ac9693ddc4e4521c0abed47db76b8f08d))

## 1.0.0 (2026-05-07)


### Bug Fixes

* **state:** promote modernc.org/sqlite to direct dep in go.mod ([2e0b349](https://github.com/rebelopsio/archy/commit/2e0b3490df5be55d4f6b49c525af90f6170102e3))
