import pc from 'picocolors';

export function banner(title: string): void {
  console.log(pc.bold(pc.cyan(`\n${title}\n`)));
}

export function checkOk(message: string): void {
  console.log(pc.green('[check]'), message);
}

export function checkWarn(message: string): void {
  console.log(pc.yellow('[check]'), message);
}

export function checkFail(message: string): void {
  console.log(pc.red('[check]'), message);
}

export function tagLine(tag: string, message: string, color: (s: string) => string = pc.blue): void {
  console.log(color(`[${tag}]`), message);
}
