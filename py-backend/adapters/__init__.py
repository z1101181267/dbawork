"""数据库适配器包。

base.py 定义统一接口；各 *_adapter.py 实现具体数据库连接；
registry.py 维护 db_type -> 适配器映射（含兼容数据库别名）。
"""
