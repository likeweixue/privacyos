import { defineStore } from 'pinia'

let seq = 0
export const useTaskStore = defineStore('tasks', {
  state: () => ({ tasks: [] }),
  getters: {
    running: (state) => state.tasks.filter((t) => t.status === 'running').length
  },
  actions: {
    add(label, detail = '') {
      const task = { id: `task-${++seq}`, label, detail, status: 'running', createdAt: new Date().toISOString() }
      this.tasks.unshift(task)
      if (this.tasks.length > 50) this.tasks = this.tasks.slice(0, 50)
      return task
    },
    finish(task, status = 'done', message = '') {
      if (!task) return
      task.status = status
      task.message = message
      task.finishedAt = new Date().toISOString()
    },
    clear() {
      this.tasks = []
    }
  }
})
