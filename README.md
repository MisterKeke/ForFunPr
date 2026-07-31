# Something C#

Something C# is a native Windows desktop application built with C#, .NET 10, and WPF. It is a rewrite of the original Something application, organized around separate domain, application, infrastructure, and desktop UI projects.

## Features

- Tasks, subtasks, priorities, due dates, and filtering
- Notes and bookmarks
- Favorite categories and update tracking
- Weather and currency information
- Telegram and YouTube content views
- File browsing
- Wallpaper selection and management
- Local SQLite persistence

## Requirements

- Windows 10 or Windows 11
- [.NET 10 SDK](https://dotnet.microsoft.com/download/dotnet/10.0)
- Visual Studio with the **.NET desktop development** workload, or another editor that supports .NET projects

## Repository structure

```text
SomethingCSharp.slnx
src/
  Something.Domain/          Domain models, validation, and exceptions
  Something.Application/     Application services and abstractions
  Something.Infrastructure/  SQLite, external providers, and OS integrations
  Something.Desktop/         WPF views, view models, themes, and startup
tests/
  Something.Infrastructure.Tests/
```

## Getting started

Clone the independent C# branch:

```powershell
git clone --branch csharp-rewrite --single-branch https://github.com/MisterKeke/ForFunPr.git
Set-Location ForFunPr
```

Restore dependencies and run the desktop application:

```powershell
dotnet restore SomethingCSharp.slnx
dotnet run --project src/Something.Desktop/Something.Desktop.csproj
```

## Build and test

```powershell
dotnet build SomethingCSharp.slnx
dotnet test tests/Something.Infrastructure.Tests/Something.Infrastructure.Tests.csproj
```

## Local application data

Development data is stored outside the repository under:

```text
%APPDATA%\Something.CSharp.Dev
```

This directory can contain the SQLite database, logs, and user-selected wallpapers. To use another location during development, set the `SOMETHING_CSHARP_DEV_DATA` environment variable.

Runtime data, local configuration, credentials, and generated build output should not be committed. See [`.gitignore`](.gitignore) for the repository exclusions.

## License

No license has been selected yet. Add a license before distributing the project or accepting external contributions.
