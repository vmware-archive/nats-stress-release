class Lib
  def flat_hash(prefix, h)
    flat = flat_hash_inner(h)
    strings = {}
    flat.each do |k,v|
      strings[prefix + "." + k.join('.')] = v
    end
    strings
  end

  def transform(h, &blk)
    if h.is_a? Hash
      new_h = {}
      h.each do |k,v|
        new_key, new_val = blk.call(k,v)
        new_h[new_key] = transform(new_val, &blk)
      end
      new_h
    elsif h.is_a? Array
      h.map.with_index do |e,i|
        _, new_val = blk.call(i,e)
        transform(new_val, &blk)
      end
    else
      h
    end
  end

  def flat_hash_inner(h,f=[],g={})
    return g.update({ f=>h }) unless h.is_a? Hash
    h.each { |k,r| flat_hash_inner(r,f+[k],g) }
    g
  end
end

Proc.new do |k,v|
  if v.is_a? Array
    [v["ip"], v]
  end
end
