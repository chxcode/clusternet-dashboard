import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { PropagationFilterControls } from './PropagationFilterControls';

describe('PropagationFilterControls', () => {
  it('emits selected propagation filter', () => {
    const onChange = vi.fn();
    render(<PropagationFilterControls value="all" onChange={onChange} />);

    fireEvent.change(screen.getByLabelText('按分发状态筛选'), { target: { value: 'online-not-observed' } });

    expect(onChange).toHaveBeenCalledWith('online-not-observed');
  });
});
