using Something.Application.Features.Tasks;
using Something.Domain.Enums;
using Something.Domain.Exceptions;
using Something.Domain.Models.Tasks;

namespace Something.Infrastructure.Tests;

public sealed class TaskRepositoryTests
{
    [Fact]
    public async Task ExistingTaskReturnsEmptyMetadata()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await TaskRepositoryTestHarness.CreateAsync();
        await using (var connection = await harness.ConnectionFactory.OpenConnectionAsync(cancellationToken))
        await using (var command = connection.CreateCommand())
        {
            command.CommandText = "INSERT INTO todos (title) VALUES ($title);";
            command.Parameters.AddWithValue("$title", "Existing task");
            await command.ExecuteNonQueryAsync(cancellationToken);
        }

        var tasks = await harness.Service.SearchAsync(cancellationToken: cancellationToken);

        var task = Assert.Single(tasks);
        Assert.Null(task.Difficulty);
        Assert.Empty(task.Tags);
        Assert.Empty(task.Subtasks);
    }

    [Fact]
    public async Task HardTaskMetadataIsPreservedValidatedAndToggleable()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await TaskRepositoryTestHarness.CreateAsync();
        var created = await harness.Service.CreateAsync(new CreateTaskRequest(
            "Ship release",
            string.Empty,
            TaskPriority.Medium,
            null,
            TaskDifficulty.Hard,
            ["Release", "release", "Backend"],
            [
                new TaskSubtaskInput(0, "Prepare changelog", false),
                new TaskSubtaskInput(0, "Publish binaries", false),
            ]), cancellationToken);

        Assert.Equal(TaskDifficulty.Hard, created.Difficulty);
        Assert.Equal(2, created.Tags.Count);
        Assert.Equal(2, created.Subtasks.Count);

        var updated = await harness.Service.UpdateAsync(new UpdateTaskRequest(
            created.Id,
            "Ship stable release",
            created.Description,
            created.Priority,
            created.DueDate,
            created.Difficulty,
            created.Tags,
            created.Subtasks.Select((subtask, position) => new TaskSubtaskInput(
                subtask.Id, subtask.Title, subtask.IsCompleted, position)).ToArray()),
            cancellationToken);

        Assert.Equal(2, updated.Tags.Count);
        Assert.Equal(2, updated.Subtasks.Count);

        await harness.Service.ToggleSubtaskAsync(updated.Id, updated.Subtasks[0].Id, cancellationToken);
        var toggled = Assert.Single(await harness.Service.SearchAsync(cancellationToken: cancellationToken));
        Assert.True(toggled.Subtasks[0].IsCompleted);

        await Assert.ThrowsAsync<ValidationException>(() => harness.Service.UpdateAsync(
            new UpdateTaskRequest(
                toggled.Id,
                toggled.Title,
                toggled.Description,
                toggled.Priority,
                toggled.DueDate,
                TaskDifficulty.Medium,
                toggled.Tags,
                toggled.Subtasks.Select((subtask, position) => new TaskSubtaskInput(
                    subtask.Id, subtask.Title, subtask.IsCompleted, position)).ToArray()),
            cancellationToken));

        var cleared = await harness.Service.UpdateAsync(new UpdateTaskRequest(
            toggled.Id,
            toggled.Title,
            toggled.Description,
            toggled.Priority,
            toggled.DueDate,
            TaskDifficulty.Medium,
            toggled.Tags,
            []),
            cancellationToken);
        Assert.Equal(TaskDifficulty.Medium, cleared.Difficulty);
        Assert.Empty(cleared.Subtasks);
    }

    [Fact]
    public async Task SearchCombinesTextMetadataAndExactFilters()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await TaskRepositoryTestHarness.CreateAsync();
        await harness.Service.CreateAsync(new CreateTaskRequest(
            "Ship release", "Prepare the stable rollout", TaskPriority.High,
            new DateOnly(2026, 8, 1), TaskDifficulty.Hard,
            ["Backend", "Urgent"], [new TaskSubtaskInput(0, "Publish binaries", false)]), cancellationToken);
        await harness.Service.CreateAsync(new CreateTaskRequest(
            "Write documentation", string.Empty, TaskPriority.Low,
            new DateOnly(2026, 8, 1), TaskDifficulty.Easy, ["Docs", "Urgent"], []), cancellationToken);
        await harness.Service.CreateAsync(new CreateTaskRequest(
            "Triage backlog", string.Empty, TaskPriority.Medium,
            null, null, ["Backend"], []), cancellationToken);
        await harness.Service.CreateAsync(new CreateTaskRequest(
            "Reach 100% coverage", string.Empty, TaskPriority.Medium,
            null, null, [], []), cancellationToken);

        var subtaskMatch = Assert.Single(await harness.Service.SearchAsync(
            TaskSearchCriteria.Empty with { Query = "binaries" }, cancellationToken));
        Assert.Equal("Ship release", subtaskMatch.Title);

        var combinedMatch = Assert.Single(await harness.Service.SearchAsync(new TaskSearchCriteria(
            "rollout", new DateOnly(2026, 8, 1), TaskPriority.High,
            TaskDifficulty.Hard, false, ["backend", "URGENT"]), cancellationToken));
        Assert.Equal("Ship release", combinedMatch.Title);

        var unsetMatch = Assert.Single(await harness.Service.SearchAsync(
            TaskSearchCriteria.Empty with { DifficultyIsUnset = true, Tags = ["backend"] }, cancellationToken));
        Assert.Equal("Triage backlog", unsetMatch.Title);

        var wildcardMatch = Assert.Single(await harness.Service.SearchAsync(
            TaskSearchCriteria.Empty with { Query = "%" }, cancellationToken));
        Assert.Equal("Reach 100% coverage", wildcardMatch.Title);
    }

    [Fact]
    public async Task MutationsRejectUnknownTasksAndSubtasks()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await TaskRepositoryTestHarness.CreateAsync();

        await Assert.ThrowsAsync<NotFoundException>(() => harness.Service.ToggleAsync(99, cancellationToken));
        await Assert.ThrowsAsync<NotFoundException>(() => harness.Service.ToggleSubtaskAsync(99, 100, cancellationToken));
        await Assert.ThrowsAsync<NotFoundException>(() => harness.Service.DeleteAsync(99, cancellationToken));
    }
}
