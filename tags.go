package serfer

// BindTags assign a map of tags to the clusters tags
func (s *Serfer) BindTags(tags map[string]string) {
	s.Conf.Tags = tags
}

// SetTag add one tag to the clusters tags
func (s *Serfer) SetTag(name, value string) {
	s.Conf.Tags[name] = value
	if s.cluster != nil {
		tags := s.Tags()
		tags[name] = value
		s.cluster.SetTags(tags)
	}
}

// Tags returns all the clusters tags
func (s *Serfer) Tags() map[string]string {
	if s.cluster != nil {
		return s.cluster.LocalMember().Tags
	}
	return s.Conf.Tags
}

// Tag return the value of a named tag. Returning if the tag exists or not
func (s *Serfer) Tag(name string) (string, bool) {
	if s.cluster != nil {
		tags := s.Tags()
		v, ok := tags[name]
		return v, ok
	}
	v, ok := s.Conf.Tags[name]
	return v, ok
}
