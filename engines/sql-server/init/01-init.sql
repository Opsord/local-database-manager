DECLARE @db SYSNAME = N'$(DB)';
DECLARE @schema SYSNAME = N'$(SCHEMA)';

IF DB_ID(@db) IS NULL
BEGIN
    DECLARE @createDbSql NVARCHAR(MAX) = N'CREATE DATABASE ' + QUOTENAME(@db);
    EXEC(@createDbSql);
END
GO

DECLARE @dbName SYSNAME = N'$(DB)';
DECLARE @schemaName SYSNAME = N'$(SCHEMA)';
DECLARE @useAndCreateSchema NVARCHAR(MAX) =
    N'USE ' + QUOTENAME(@dbName) + N'; ' +
    N'IF NOT EXISTS (SELECT 1 FROM sys.schemas WHERE name = N''' + REPLACE(@schemaName, N'''', N'''''') + N''') ' +
    N'EXEC(N''CREATE SCHEMA '' + QUOTENAME(N''' + REPLACE(@schemaName, N'''', N'''''') + N'''));';
EXEC(@useAndCreateSchema);
GO
